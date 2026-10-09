package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/frain-dev/convoy/config"
)

// Namespace used in fully-qualified metrics names.
const namespace = "convoy"

var (
	metricsMu       sync.Mutex
	lastRun         time.Time
	cachedMetrics   *Metrics
	metricsInFlight bool
	metricsConfig   *config.MetricsConfiguration
)

type EventQueueMetrics struct {
	ProjectID string `json:"project_id" db:"project_id"`
	SourceId  string `json:"source_id" db:"source_id"`
	Total     uint64 `json:"total" db:"total"`
}

type EventQueueBacklogMetrics struct {
	ProjectID  string  `json:"project_id" db:"project_id"`
	SourceId   string  `json:"source_id" db:"source_id"`
	AgeSeconds float64 `json:"age_seconds" db:"age_seconds"`
}

type EventDeliveryQueueMetrics struct {
	ProjectID        string `json:"project_id" db:"project_id"`
	ProjectName      string `json:"project_name" db:"project_name"`
	EndpointId       string `json:"endpoint_id" db:"endpoint_id"`
	Status           string `json:"status" db:"status"`
	EventType        string `json:"event_type" db:"event_type"`
	SourceId         string `json:"source_id" db:"source_id"`
	OrganisationID   string `json:"organisation_id" db:"organisation_id"`
	OrganisationName string `json:"organisation_name" db:"organisation_name"`
	Total            uint64 `json:"total" db:"total"`
}

type EventDeliveryQueueLatencyMetrics struct {
	ProjectID      string `json:"project_id" db:"project_id"`
	EndpointId     string `json:"endpoint_id" db:"endpoint_id"`
	Status         string `json:"status" db:"status"`
	LatencySeconds uint64 `json:"latency_seconds" db:"latency_seconds"`
}

type EventQueueEndpointBacklogMetrics struct {
	ProjectID  string  `json:"project_id" db:"project_id"`
	SourceId   string  `json:"source_id" db:"source_id"`
	EndpointId string  `json:"endpoint_id" db:"endpoint_id"`
	AgeSeconds float64 `json:"age_seconds" db:"age_seconds"`
}

type EventQueueEndpointAttemptMetrics struct {
	ProjectID  string `json:"project_id" db:"project_id"`
	EndpointId string `json:"endpoint_id" db:"endpoint_id"`
	Status     string `json:"status" db:"status"`
	StatusCode string `json:"status_code" db:"status_code"`
	Total      uint64 `json:"total" db:"total"`
}

type Metrics struct {
	EventQueueMetrics []EventQueueMetrics

	//
	EventQueueBacklogMetrics         []EventQueueBacklogMetrics
	EventDeliveryQueueMetrics        []EventDeliveryQueueMetrics
	EventQueueEndpointBacklogMetrics []EventQueueEndpointBacklogMetrics
	EventQueueEndpointAttemptMetrics []EventQueueEndpointAttemptMetrics
}

var (
	eventQueueTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_queue_total"),
		"Total number of tasks in the event queue",
		[]string{"project", "source", "status"}, nil,
	)

	eventQueueBacklogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_queue_backlog_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed.",
		[]string{"project", "source"}, nil,
	)

	eventDeliveryQueueTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_queue_total"),
		"Total number of tasks in the delivery queue per endpoint",
		[]string{"project", "project_name", "endpoint", "status", "event_type", "source", "organisation_id", "organisation_name"}, nil,
	)

	eventDeliveryQueueBacklogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_queue_backlog_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed per endpoint",
		[]string{"project", "endpoint", "source"}, nil,
	)

	eventDeliveryAttemptsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_attempts_total"),
		"Total number of attempts per endpoint",
		[]string{"project", "endpoint", "status", "http_status_code"}, nil,
	)
)

func (p *Postgres) Describe(ch chan<- *prometheus.Desc) {
	// Static descriptors only. DescribeByCollect ran collectMetrics during
	// prometheus.Register, which sits on the server and agent boot path before
	// listen/Asynq. The delivery-queue GROUP BY can take the full QueryTimeout
	// (default 30s) when the materialized views are absent.
	ch <- eventQueueTotalDesc
	ch <- eventQueueBacklogDesc
	ch <- eventDeliveryQueueTotalDesc
	ch <- eventDeliveryQueueBacklogDesc
	ch <- eventDeliveryAttemptsTotalDesc
}

func (p *Postgres) Collect(ch chan<- prometheus.Metric) {
	if !p.metricsEnabled() {
		return
	}

	snapshot := p.cachedSnapshot()
	p.scheduleRefresh()
	p.emitMetrics(ch, snapshot)
}

// StartMetricsCollection kicks the first refresh off the boot path after
// Register. Failure policy: fail open. The scrape serves whatever cache exists
// (empty until the first refresh finishes); ingest and listen must not wait.
func (p *Postgres) StartMetricsCollection() {
	if !p.metricsEnabled() {
		return
	}
	p.scheduleRefresh()
}

func (p *Postgres) metricsEnabled() bool {
	cfg := p.loadMetricsConfig()
	return cfg != nil && cfg.IsEnabled
}

func (p *Postgres) loadMetricsConfig() *config.MetricsConfiguration {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if metricsConfig != nil {
		return metricsConfig
	}
	cfg, err := config.Get()
	if err != nil {
		return nil
	}
	metricsConfig = &cfg.Metrics
	return metricsConfig
}

func (p *Postgres) cachedSnapshot() *Metrics {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if cachedMetrics == nil {
		return &Metrics{}
	}
	return cachedMetrics
}

func (p *Postgres) scheduleRefresh() {
	cfg := p.loadMetricsConfig()
	if cfg == nil || !cfg.IsEnabled {
		return
	}
	sample := time.Duration(cfg.Prometheus.SampleTime) * time.Second

	metricsMu.Lock()
	fresh := !lastRun.IsZero() && time.Since(lastRun) < sample
	if fresh || metricsInFlight {
		metricsMu.Unlock()
		return
	}
	metricsInFlight = true
	metricsMu.Unlock()

	go p.refreshMetrics()
}

func (p *Postgres) refreshMetrics() {
	defer func() {
		if rec := recover(); rec != nil {
			p.metricsLogError(fmt.Sprintf("postgres metrics refresh panicked: %v", rec))
		}
		metricsMu.Lock()
		metricsInFlight = false
		metricsMu.Unlock()
	}()

	metrics, err := p.collectMetrics()
	metricsMu.Lock()
	defer metricsMu.Unlock()
	lastRun = time.Now()
	if err != nil {
		p.metricsLogError(fmt.Sprintf("Failed to collect metrics data: %v", err))
		return
	}
	cachedMetrics = metrics
}

func (p *Postgres) metricsLogError(msg string) {
	if p != nil && p.logger != nil {
		p.logger.Error(msg)
	}
}

func (p *Postgres) emitMetrics(ch chan<- prometheus.Metric, metrics *Metrics) {
	if metrics == nil {
		return
	}

	// Use unique keys per metric type to prevent collisions
	metricsMap := make(map[string]struct{})

	for _, metric := range metrics.EventQueueMetrics {
		key := fmt.Sprintf("event_queue_total_%s_%s", metric.ProjectID, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
			p.logger.Warn(fmt.Sprintf("Duplicate metric detected and skipped: event_queue_total (project: %s, source: %s)", metric.ProjectID, metric.SourceId))
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventQueueTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.SourceId,
			"success", // already in db
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventQueueBacklogMetrics {
		key := fmt.Sprintf("event_queue_backlog_%s_%s", metric.ProjectID, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
			p.logger.Warn(fmt.Sprintf("Duplicate metric detected and skipped: event_queue_backlog (project: %s, source: %s)", metric.ProjectID, metric.SourceId))
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventQueueBacklogDesc,
			prometheus.GaugeValue,
			metric.AgeSeconds,
			metric.ProjectID,
			metric.SourceId,
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventDeliveryQueueMetrics {
		key := fmt.Sprintf("event_delivery_queue_total_%s_%s_%s_%s_%s_%s", metric.ProjectID, metric.EndpointId, strings.ToLower(metric.Status), metric.EventType, metric.SourceId, metric.OrganisationID)
		if _, ok := metricsMap[key]; ok {
			p.logger.Warn(fmt.Sprintf("Duplicate metric detected and skipped: event_delivery_queue_total (project: %s, endpoint: %s, status: %s, event_type: %s, source: %s, organisation_id: %s)", metric.ProjectID, metric.EndpointId, metric.Status, metric.EventType, metric.SourceId, metric.OrganisationID))
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.ProjectName,
			metric.EndpointId,
			strings.ToLower(metric.Status),
			metric.EventType,
			metric.SourceId,
			metric.OrganisationID,
			metric.OrganisationName,
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventQueueEndpointBacklogMetrics {
		key := fmt.Sprintf("event_delivery_queue_backlog_%s_%s_%s", metric.ProjectID, metric.EndpointId, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
			p.logger.Warn(fmt.Sprintf("Duplicate metric detected and skipped: event_delivery_queue_backlog (project: %s, endpoint: %s, source: %s)", metric.ProjectID, metric.EndpointId, metric.SourceId))
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueBacklogDesc,
			prometheus.GaugeValue,
			metric.AgeSeconds,
			metric.ProjectID,
			metric.EndpointId,
			metric.SourceId,
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventQueueEndpointAttemptMetrics {
		key := fmt.Sprintf("event_delivery_attempts_total_%s_%s_%s_%s", metric.ProjectID, metric.EndpointId, strings.ToLower(metric.Status), metric.StatusCode)
		if _, ok := metricsMap[key]; ok {
			p.logger.Warn(fmt.Sprintf("Duplicate metric detected and skipped: event_delivery_attempts_total (project: %s, endpoint: %s, status: %s, status_code: %s)", metric.ProjectID, metric.EndpointId, metric.Status, metric.StatusCode))
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryAttemptsTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.EndpointId,
			strings.ToLower(metric.Status),
			metric.StatusCode,
		)
		metricsMap[key] = struct{}{}
	}
}

// collectMetrics reads the current snapshot generation. The live GROUP BY
// lives in WriteQueueMetricsSnapshot, which a locked worker runs off scrape.
func (p *Postgres) collectMetrics() (*Metrics, error) {
	queryTimeout := 30 * time.Second
	if cfg := p.loadMetricsConfig(); cfg != nil && cfg.Prometheus.QueryTimeout != 0 {
		queryTimeout = time.Duration(cfg.Prometheus.QueryTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	metrics := &Metrics{}

	rows, err := p.GetDB().QueryxContext(ctx, `
		SELECT q.project_id, q.source_id, q.total
		FROM convoy.metrics_event_queue q
		JOIN convoy.metrics_snapshot_meta m
		  ON m.name = $1 AND m.generation = q.generation`, metricsSnapshotEventQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to query event queue metrics: %w", err)
	}
	defer closeWithError(rows)
	eventQueueMetrics := make([]EventQueueMetrics, 0)
	for rows.Next() {
		var eqm EventQueueMetrics
		err = rows.StructScan(&eqm)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event queue metrics: %w", err)
		}
		eventQueueMetrics = append(eventQueueMetrics, eqm)
	}
	metrics.EventQueueMetrics = eventQueueMetrics

	rows1, err := p.GetDB().QueryxContext(ctx, `
		SELECT q.project_id, q.source_id, q.age_seconds
		FROM convoy.metrics_event_queue_backlog q
		JOIN convoy.metrics_snapshot_meta m
		  ON m.name = $1 AND m.generation = q.generation`, metricsSnapshotEventQueueBacklog)
	if err != nil {
		return nil, fmt.Errorf("failed to query backlog metrics: %w", err)
	}
	defer closeWithError(rows1)
	eventQueueBacklogMetrics := make([]EventQueueBacklogMetrics, 0)
	for rows1.Next() {
		var e EventQueueBacklogMetrics
		err = rows1.StructScan(&e)
		if err != nil {
			return nil, err
		}
		eventQueueBacklogMetrics = append(eventQueueBacklogMetrics, e)
	}
	metrics.EventQueueBacklogMetrics = eventQueueBacklogMetrics

	rows2, err := p.GetDB().QueryxContext(ctx, `
		SELECT q.project_id, q.project_name, q.endpoint_id, q.status, q.event_type,
		       q.source_id, q.organisation_id, q.organisation_name, q.total
		FROM convoy.metrics_event_delivery_queue q
		JOIN convoy.metrics_snapshot_meta m
		  ON m.name = $1 AND m.generation = q.generation`, metricsSnapshotEventDeliveryQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to query delivery queue metrics: %w", err)
	}
	defer closeWithError(rows2)
	eventDeliveryQueueMetrics := make([]EventDeliveryQueueMetrics, 0)
	for rows2.Next() {
		var eqm EventDeliveryQueueMetrics
		err = rows2.StructScan(&eqm)
		if err != nil {
			return nil, err
		}
		eventDeliveryQueueMetrics = append(eventDeliveryQueueMetrics, eqm)
	}
	metrics.EventDeliveryQueueMetrics = eventDeliveryQueueMetrics

	rows3, err := p.GetDB().QueryxContext(ctx, `
		SELECT q.project_id, q.source_id, q.endpoint_id, q.age_seconds
		FROM convoy.metrics_event_endpoint_backlog q
		JOIN convoy.metrics_snapshot_meta m
		  ON m.name = $1 AND m.generation = q.generation`, metricsSnapshotEventEndpointBacklog)
	if err != nil {
		return nil, fmt.Errorf("failed to query endpoint backlog metrics: %w", err)
	}
	defer closeWithError(rows3)
	eventQueueEndpointBacklogMetrics := make([]EventQueueEndpointBacklogMetrics, 0)
	for rows3.Next() {
		var e EventQueueEndpointBacklogMetrics
		err = rows3.StructScan(&e)
		if err != nil {
			return nil, err
		}
		eventQueueEndpointBacklogMetrics = append(eventQueueEndpointBacklogMetrics, e)
	}
	metrics.EventQueueEndpointBacklogMetrics = eventQueueEndpointBacklogMetrics

	return metrics, nil
}
