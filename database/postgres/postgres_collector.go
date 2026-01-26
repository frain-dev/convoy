package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
)

// Namespace used in fully-qualified metrics names.
const namespace = "convoy"

var lastRun = time.Now()

var cachedMetrics *Metrics // needed to feed the UI with data when sampling time has not yet elapsed

var metricsConfig *config.MetricsConfiguration

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
	prometheus.DescribeByCollect(p, ch)
}

func (p *Postgres) Collect(ch chan<- prometheus.Metric) {
	if metricsConfig == nil {
		cfg, err := config.Get()
		if err != nil {
			return
		}
		metricsConfig = &cfg.Metrics
	}
	if !metricsConfig.IsEnabled {
		return
	}

	var metrics *Metrics
	var err error
	now := time.Now()
	if cachedMetrics != nil && lastRun.Add(time.Duration(metricsConfig.Prometheus.SampleTime)*time.Second).After(now) {
		metrics = cachedMetrics
	} else {
		metrics, err = p.collectMetrics()
		if err != nil {
			log.Errorf("Failed to collect metrics data: %v", err)
			if cachedMetrics != nil {
				metrics = cachedMetrics
				log.Warn("Using cached metrics due to collection failure")
			} else {
				// Return empty metrics to prevent blocking the endpoint
				metrics = &Metrics{}
			}
		}
		cachedMetrics = metrics
	}

	// Use unique keys per metric type to prevent collisions
	metricsMap := make(map[string]struct{})

	for _, metric := range metrics.EventQueueMetrics {
		key := fmt.Sprintf("event_queue_total_%s_%s", metric.ProjectID, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
			log.Warnf("Duplicate metric detected and skipped: event_queue_total (project: %s, source: %s)", metric.ProjectID, metric.SourceId)
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
			log.Warnf("Duplicate metric detected and skipped: event_queue_backlog (project: %s, source: %s)", metric.ProjectID, metric.SourceId)
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
			log.Warnf("Duplicate metric detected and skipped: event_delivery_queue_total (project: %s, endpoint: %s, status: %s, event_type: %s, source: %s, organisation_id: %s)", metric.ProjectID, metric.EndpointId, metric.Status, metric.EventType, metric.SourceId, metric.OrganisationID)
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
			log.Warnf("Duplicate metric detected and skipped: event_delivery_queue_backlog (project: %s, endpoint: %s, source: %s)", metric.ProjectID, metric.EndpointId, metric.SourceId)
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
			log.Warnf("Duplicate metric detected and skipped: event_delivery_attempts_total (project: %s, endpoint: %s, status: %s, status_code: %s)", metric.ProjectID, metric.EndpointId, metric.Status, metric.StatusCode)
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

	lastRun = now
}

// materializedViewExists checks if a materialized view exists in the database
func (p *Postgres) materializedViewExists(ctx context.Context, viewName string) bool {
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM pg_matviews 
			WHERE schemaname = 'convoy' 
			AND matviewname = $1
		)`
	var exists bool
	err := p.GetDB().GetContext(ctx, &exists, query, viewName)
	if err != nil {
		log.Warnf("Failed to check if materialized view %s exists: %v", viewName, err)
		return false
	}
	return exists
}

// collectMetrics gathers essential metrics from the DB
func (p *Postgres) collectMetrics() (*Metrics, error) {
	queryTimeout := time.Duration(metricsConfig.Prometheus.QueryTimeout) * time.Second
	if queryTimeout == 0 {
		queryTimeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	metrics := &Metrics{}

	useMaterializedViews := p.materializedViewExists(ctx, "event_queue_metrics_mv")

	var queryEventQueueMetrics string
	if useMaterializedViews {
		queryEventQueueMetrics = "SELECT project_id, source_id, total FROM convoy.event_queue_metrics_mv"
	} else {
		queryEventQueueMetrics = `
			SELECT DISTINCT 
				project_id,
				COALESCE(source_id, 'http') AS source_id,
				COUNT(*) AS total
			FROM convoy.events
			GROUP BY project_id, source_id`
	}
	rows, err := p.GetDB().QueryxContext(ctx, queryEventQueueMetrics)
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

	var backlogQM string
	if useMaterializedViews {
		backlogQM = "SELECT project_id, source_id, age_seconds FROM convoy.event_queue_backlog_metrics_mv"
	} else {
		backlogQM = `
			WITH a1 AS (
				SELECT ed.project_id,
					   COALESCE(e.source_id, 'http') AS source_id,
					   EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
				FROM convoy.event_deliveries ed
				LEFT JOIN convoy.events e ON e.id = ed.event_id
				WHERE ed.status = 'Processing'
				GROUP BY ed.project_id, e.source_id
				ORDER BY age_seconds DESC, ed.project_id, e.source_id
				LIMIT 1000
			)
			SELECT project_id, source_id, age_seconds
			FROM (
				SELECT * FROM a1
				UNION ALL
				SELECT ed.project_id,
					   COALESCE(e.source_id, 'http'),
					   0 AS age_seconds
				FROM convoy.event_deliveries ed
				LEFT JOIN convoy.events e ON e.id = ed.event_id
				WHERE ed.status = 'Success'
				  AND NOT EXISTS (
					  SELECT 1 FROM a1 
					  WHERE a1.project_id = ed.project_id 
						AND a1.source_id = COALESCE(e.source_id, 'http')
				  )
				GROUP BY ed.project_id, e.source_id
			) AS combined
			ORDER BY project_id, source_id
			LIMIT 1000`
	}
	rows1, err := p.GetDB().QueryxContext(ctx, backlogQM)
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

	var queryDeliveryQ string
	if useMaterializedViews {
		queryDeliveryQ = `SELECT 
			project_id, 
			project_name,
			endpoint_id, 
			status,
			event_type,
			source_id,
			organisation_id,
			organisation_name,
			total 
		FROM convoy.event_delivery_queue_metrics_mv`
	} else {
		queryDeliveryQ = `
			SELECT DISTINCT 
				ed.project_id,
				COALESCE(p.name, '') AS project_name,
				ed.endpoint_id,
				ed.status,
				COALESCE(ed.event_type, '') AS event_type,
				COALESCE(e.source_id, 'http') AS source_id,
				COALESCE(p.organisation_id, '') AS organisation_id,
				COALESCE(o.name, '') AS organisation_name,
				COUNT(*) AS total
			FROM convoy.event_deliveries ed
			LEFT JOIN convoy.events e ON ed.event_id = e.id
			LEFT JOIN convoy.projects p ON ed.project_id = p.id
			LEFT JOIN convoy.organisations o ON p.organisation_id = o.id
			WHERE ed.deleted_at IS NULL
			GROUP BY ed.project_id, p.name, ed.endpoint_id, ed.status, ed.event_type, e.source_id, p.organisation_id, o.name`
	}
	rows2, err := p.GetDB().QueryxContext(ctx, queryDeliveryQ)
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

	var backlogEQM string
	if useMaterializedViews {
		backlogEQM = "SELECT project_id, source_id, endpoint_id, age_seconds FROM convoy.event_endpoint_backlog_metrics_mv"
	} else {
		backlogEQM = `
			WITH a1 AS (
				SELECT ed.project_id,
					   COALESCE(e.source_id, 'http') AS source_id,
					   ed.endpoint_id,
					   EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
				FROM convoy.event_deliveries ed
				LEFT JOIN convoy.events e ON e.id = ed.event_id
				WHERE ed.status = 'Processing'
				GROUP BY ed.project_id, e.source_id, ed.endpoint_id
				ORDER BY age_seconds DESC, ed.project_id, e.source_id, ed.endpoint_id
				LIMIT 1000
			)
			SELECT project_id, source_id, endpoint_id, age_seconds
			FROM (
				SELECT * FROM a1
				UNION ALL
				SELECT ed.project_id,
					   COALESCE(e.source_id, 'http'),
					   ed.endpoint_id,
					   0 AS age_seconds
				FROM convoy.event_deliveries ed
				LEFT JOIN convoy.events e ON e.id = ed.event_id
				WHERE ed.status = 'Success'
				  AND NOT EXISTS (
					  SELECT 1 FROM a1 
					  WHERE a1.project_id = ed.project_id 
						AND a1.source_id = COALESCE(e.source_id, 'http')
						AND a1.endpoint_id = ed.endpoint_id
				  )
				GROUP BY ed.project_id, e.source_id, ed.endpoint_id
			) AS combined
			ORDER BY project_id, source_id, endpoint_id
			LIMIT 1000`
	}
	rows3, err := p.GetDB().QueryxContext(ctx, backlogEQM)
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
