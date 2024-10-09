package metrics

import (
	"sync"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	m    *Metrics
	once sync.Once
)

const (
	projectLabel  = "project"
	sourceLabel   = "source"
	endpointLabel = "endpoint"
)

// Metrics for the data plane
type Metrics struct {
	IsEnabled            bool
	IngestTotal          *prometheus.CounterVec
	IngestConsumedTotal  *prometheus.CounterVec
	IngestErrorsTotal    *prometheus.CounterVec
	IngestLatency        *prometheus.HistogramVec
	EventDeliveryLatency *prometheus.HistogramVec
}

func GetDPInstance(licenser license.Licenser) *Metrics {
	once.Do(func() {
		m = newMetrics(Reg(), licenser)
	})
	return m
}

func newMetrics(pr prometheus.Registerer, licenser license.Licenser) *Metrics {
	m := InitMetrics(licenser)

	if m.IsEnabled && m.IngestTotal != nil && m.IngestConsumedTotal != nil && m.IngestErrorsTotal != nil {
		pr.MustRegister(
			m.IngestTotal,
			m.IngestConsumedTotal,
			m.IngestErrorsTotal,
			m.EventDeliveryLatency,
		)
	}
	return m
}

func InitMetrics(licenser license.Licenser) *Metrics {
	cfg, err := config.Get()
	if err != nil {
		return &Metrics{
			IsEnabled: false,
		}
	}
	if !cfg.Metrics.IsEnabled || !licenser.CanExportPrometheusMetrics() {
		return &Metrics{
			IsEnabled: false,
		}
	}

	m := &Metrics{
		IsEnabled: true,

		IngestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_ingest_total",
				Help: "Total number of events ingested",
			},
			[]string{projectLabel, sourceLabel},
		),
		IngestConsumedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_ingest_success",
				Help: "Total number of events successfully ingested and consumed",
			},
			[]string{projectLabel, sourceLabel},
		),
		IngestErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_ingest_error",
				Help: "Total number of errors during event ingestion",
			},
			[]string{projectLabel, sourceLabel},
		),
		IngestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_ingest_latency",
				Help:    "Total time (in seconds) an event spends in Convoy.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{projectLabel},
		),
		EventDeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_end_to_end_latency",
				Help:    "Total time (in seconds) an event spends in Convoy.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{projectLabel, endpointLabel},
		),
	}
	return m
}

func (m *Metrics) RecordEndToEndLatency(ev *datastore.EventDelivery) {
	if !m.IsEnabled {
		return
	}
	m.EventDeliveryLatency.With(prometheus.Labels{projectLabel: ev.ProjectID, endpointLabel: ev.EndpointID}).Observe(ev.LatencySeconds)
}

func (m *Metrics) RecordIngestLatency(projectId string, latency float64) {
	if !m.IsEnabled {
		return
	}
	m.IngestLatency.With(prometheus.Labels{projectLabel: projectId}).Observe(latency)
}

func (m *Metrics) IncrementIngestTotal(source string, project string) {
	if !m.IsEnabled {
		return
	}
	m.IngestTotal.With(prometheus.Labels{projectLabel: project, sourceLabel: source}).Inc()
}

func (m *Metrics) IncrementIngestConsumedTotal(source *datastore.Source) {
	if !m.IsEnabled {
		return
	}
	m.IngestConsumedTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
}

func (m *Metrics) IncrementIngestErrorsTotal(source *datastore.Source) {
	if !m.IsEnabled {
		return
	}
	m.IngestErrorsTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
}
