package postgres

import (
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

// Namespace used in fully-qualified metrics names.
const namespace = "convoy"

const delaySeconds = 1

var lastRun = time.Now()

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
		[]string{"project_id", "source", "status"}, nil,
	)

	eventQueueBacklogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_queue_backlog_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed.",
		[]string{"project_id", "source"}, nil,
	)

	eventDeliveryQueueTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "convoy_event_delivery_queue_total"),
		"Total number of tasks in the delivery queue per endpoint",
		[]string{"project_id", "endpoint", "status"}, nil,
	)

	eventDeliveryQueueBacklogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_queue_backlog_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed per endpoint",
		[]string{"project_id", "endpoint"}, nil,
	)

	eventDeliveryAttemptsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_delivery_attempts_total"),
		"Total number of attempts per endpoint",
		[]string{"project_id", "endpoint", "status", "http_status_code"}, nil,
	)
)

func (p *Postgres) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(p, ch)
}

func (p *Postgres) Collect(ch chan<- prometheus.Metric) {

	now := time.Now()
	if lastRun.Add(delaySeconds * time.Second).After(now) {
		return
	}

	metrics, err := p.collectMetrics()
	if err != nil {
		log.Printf("Failed to collect metrics data: %v", err)
		return
	}

	for _, metric := range metrics.EventQueueMetrics {
		ch <- prometheus.MustNewConstMetric(
			eventQueueTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.SourceId,
			"success", // already in db
		)
	}

	for _, metric := range metrics.EventQueueBacklogMetrics {
		ch <- prometheus.MustNewConstMetric(
			eventQueueBacklogDesc,
			prometheus.GaugeValue,
			float64(metric.AgeSeconds),
			metric.ProjectID,
			metric.SourceId,
		)
	}

	for _, metric := range metrics.EventDeliveryQueueMetrics {
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.EndpointId,
			strings.ToLower(metric.Status),
		)
	}

	for _, metric := range metrics.EventQueueEndpointBacklogMetrics {
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryQueueBacklogDesc,
			prometheus.GaugeValue,
			metric.AgeSeconds,
			metric.ProjectID,
			metric.EndpointId,
		)
	}

	for _, metric := range metrics.EventQueueEndpointAttemptMetrics {
		ch <- prometheus.MustNewConstMetric(
			eventDeliveryAttemptsTotalDesc,
			prometheus.GaugeValue,
			float64(metric.Total),
			metric.ProjectID,
			metric.EndpointId,
			strings.ToLower(metric.Status),
			metric.StatusCode,
		)
	}

	lastRun = now
}

// collectQueueInfo gathers essential metrics from the DB
func (p *Postgres) collectMetrics() (*Metrics, error) {
	metrics := &Metrics{}

	queryEventQueueMetrics := "select project_id, coalesce(source_id, 'http') as source_id, count(*) as total from events group by project_id, source_id"
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

	backlogQM := `with a1 as (
    select ed.project_id, coalesce(source_id, 'http') as source_id,
           EXTRACT(EPOCH FROM (NOW() - min(ed.created_at))) as age_seconds
    from event_deliveries ed left join convoy.events e on e.id = ed.event_id
    where status = 'Processing'
    group by ed.project_id, source_id limit 1000 --samples
    )
    select * from a1
    union all
    select ed.project_id, coalesce(source_id, 'http'), 0 as age_seconds
    from event_deliveries ed left join convoy.events e on e.id = ed.event_id
    where status = 'Success' and source_id not in (select source_id from a1)
    group by ed.project_id, source_id
    limit 1000 -- samples`
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

	queryDeliveryQ := "select project_id, endpoint_id, status, count(*) as total from event_deliveries group by project_id, endpoint_id, status"
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

	backlogEQM := `with a1 as (
    select ed.project_id, coalesce(source_id, 'http') as source_id, endpoint_id,
           EXTRACT(EPOCH FROM (NOW() - min(ed.created_at))) as age_seconds
    from event_deliveries ed left join convoy.events e on e.id = ed.event_id
    where status = 'Processing'
    group by ed.project_id, source_id, endpoint_id limit 1000 --samples
    )
    select * from a1
    union all
    select ed.project_id, coalesce(source_id, 'http'), endpoint_id, 0 as age_seconds
    from event_deliveries ed left join convoy.events e on e.id = ed.event_id
    where status = 'Success' and endpoint_id not in (select endpoint_id from a1)
    group by ed.project_id, source_id, endpoint_id
    limit 1000 -- samples`
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

	attemptsQuery := `select project_id, endpoint_id, status,
       coalesce(substring((regexp_split_to_array(convert_from(attempts, 'UTF8'), 'http_status":'))
           [array_length((regexp_split_to_array(convert_from(attempts, 'UTF8'), 'http_status":')), 1)],
           '\d{3} [A-Za-z ]{1,}'), '')
           as status_code, count(*) as total from event_deliveries group by project_id, endpoint_id, status, status_code;`
	rows4, err := p.GetDB().Queryx(attemptsQuery)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows4)
	attempts := make([]EventQueueEndpointAttemptMetrics, 0)
	for rows4.Next() {
		var e EventQueueEndpointAttemptMetrics
		err = rows4.StructScan(&e)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, e)
	}
	metrics.EventQueueEndpointAttemptMetrics = attempts

	return metrics, nil
}
