package metrics

import (
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

	pr.MustRegister(
		m.IngestTotal,
		m.IngestConsumedTotal,
		m.IngestErrorsTotal,
	)
	return m
}

func InitMetrics() *Metrics {

	m := &Metrics{

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
	m.IngestTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
}

func (m *Metrics) IncrementIngestConsumedTotal(source *datastore.Source) {
	m.IngestConsumedTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
}

func (m *Metrics) IncrementIngestErrorsTotal(source *datastore.Source) {
	m.IngestErrorsTotal.With(prometheus.Labels{projectLabel: source.ProjectID, sourceLabel: source.UID}).Inc()
}
