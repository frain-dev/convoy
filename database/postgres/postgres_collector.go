package postgres

import (
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
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
	ProjectID  string `json:"project_id" db:"project_id"`
	EndpointId string `json:"endpoint_id" db:"endpoint_id"`
	Status     string `json:"status" db:"status"`
	Total      uint64 `json:"total" db:"total"`
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
		[]string{"project", "endpoint", "status"}, nil,
	)

	eventDeliveryQueueBacklogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_queue_backlog_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed per endpoint",
		[]string{"project", "endpoint"}, nil,
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
			return
		}
		cachedMetrics = metrics
	}

	metricsMap := make(map[string]struct{})

	for _, metric := range metrics.EventQueueMetrics {
		key := fmt.Sprintf("eqm_%d_%s_%s", metric.Total, metric.ProjectID, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
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
		key := fmt.Sprintf("eqbm_%f_%s_%s", metric.AgeSeconds, metric.ProjectID, metric.SourceId)
		if _, ok := metricsMap[key]; ok {
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
		key := fmt.Sprintf("edqm_%d_%s_%s_%s", metric.Total, metric.ProjectID, metric.EndpointId, metric.Status)
		if _, ok := metricsMap[key]; ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.EndpointId,
			strings.ToLower(metric.Status),
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventQueueEndpointBacklogMetrics {
		key := fmt.Sprintf("%f_%s_%s", metric.AgeSeconds, metric.ProjectID, metric.EndpointId)
		if _, ok := metricsMap[key]; ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueBacklogDesc,
			prometheus.GaugeValue,
			metric.AgeSeconds,
			metric.ProjectID,
			metric.EndpointId,
		)
		metricsMap[key] = struct{}{}
	}

	for _, metric := range metrics.EventQueueEndpointAttemptMetrics {
		key := fmt.Sprintf("eqeam_%d_%s_%s_%s_%s", metric.Total, metric.ProjectID, metric.EndpointId, metric.Status, metric.StatusCode)
		if _, ok := metricsMap[key]; ok {
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
	clear(metricsMap)

	lastRun = now
}

// collectMetrics gathers essential metrics from the DB
func (p *Postgres) collectMetrics() (*Metrics, error) {
	metrics := &Metrics{}

	queryEventQueueMetrics := "select project_id, coalesce(source_id, 'http') as source_id, count(*) as total from convoy.events group by project_id, source_id"
	rows, err := p.GetDB().Queryx(queryEventQueueMetrics)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)
	eventQueueMetrics := make([]EventQueueMetrics, 0)
	for rows.Next() {
		var eqm EventQueueMetrics
		err = rows.StructScan(&eqm)
		if err != nil {
			return nil, err
		}
		eventQueueMetrics = append(eventQueueMetrics, eqm)
	}
	metrics.EventQueueMetrics = eventQueueMetrics

	backlogQM := `WITH a1 AS (
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http') AS source_id,
           EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
    FROM convoy.event_deliveries ed
             LEFT JOIN convoy.events e ON e.id = ed.event_id
    WHERE ed.status = 'Processing'
    GROUP BY ed.project_id, e.source_id
    LIMIT 1000 -- samples
    )
    SELECT * FROM a1
    UNION ALL
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http'),
           0 AS age_seconds
    FROM convoy.event_deliveries ed
             LEFT JOIN convoy.events e ON e.id = ed.event_id
             LEFT JOIN a1 ON e.source_id = a1.source_id
    WHERE ed.status = 'Success' AND a1.source_id IS NULL
    GROUP BY ed.project_id, e.source_id
    LIMIT 1000; -- samples`
	rows1, err := p.GetDB().Queryx(backlogQM)
	if err != nil {
		return nil, err
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

	queryDeliveryQ := "select project_id, endpoint_id, status, count(*) as total from convoy.event_deliveries group by project_id, endpoint_id, status"
	rows2, err := p.GetDB().Queryx(queryDeliveryQ)
	if err != nil {
		return nil, err
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

	backlogEQM := `WITH a1 AS (
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http') AS source_id,
           ed.endpoint_id,
           EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.events e ON e.id = ed.event_id
    WHERE ed.status = 'Processing'
    GROUP BY ed.project_id, e.source_id, ed.endpoint_id
    LIMIT 1000 -- samples
    )
    SELECT * FROM a1
    UNION ALL
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http'),
           ed.endpoint_id,
           0 AS age_seconds
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.events e ON e.id = ed.event_id
    LEFT JOIN a1 ON ed.endpoint_id = a1.endpoint_id
    WHERE ed.status = 'Success' AND a1.endpoint_id IS NULL
    GROUP BY ed.project_id, e.source_id, ed.endpoint_id
    LIMIT 1000; -- samples`
	rows3, err := p.GetDB().Queryx(backlogEQM)
	if err != nil {
		return nil, err
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
