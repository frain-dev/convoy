package metrics

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var m *Metrics
var once sync.Once

const (
	projectLabel = "project"
	sourceLabel  = "source"
)

// Metrics for the data plane
type Metrics struct {
	IsEnabled           bool
	IngestTotal         *prometheus.CounterVec
	IngestConsumedTotal *prometheus.CounterVec
	IngestErrorsTotal   *prometheus.CounterVec
}

func GetDPInstance() *Metrics {
	once.Do(func() {
		m = newMetrics(Reg())
	})
	return m
}

func newMetrics(pr prometheus.Registerer) *Metrics {
	m := InitMetrics()

	if m.IsEnabled && m.IngestTotal != nil && m.IngestConsumedTotal != nil && m.IngestErrorsTotal != nil {
		pr.MustRegister(
			m.IngestTotal,
			m.IngestConsumedTotal,
			m.IngestErrorsTotal,
		)
	}
	return m
}

func InitMetrics() *Metrics {

	cfg, err := config.Get()
	if err != nil {
		return &Metrics{
			IsEnabled: false,
		}
	}
	if !cfg.Metrics.IsEnabled {
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
	}
	return m
}

func (m *Metrics) IncrementIngestTotal(source *datastore.Source) {
	if !m.IsEnabled {
		return
	}
	m.IngestTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
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
