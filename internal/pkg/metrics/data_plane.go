package metrics

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var m *Metrics
var once sync.Once

const (
	projectLabel  = "project"
	sourceLabel   = "source"
	endpointLabel = "endpoint"
)

var (
	bucketsDefault = prometheus.LinearBuckets(1, 0.5, 20)
)

// Metrics for the data plane
type Metrics struct {
	IngestTotal         *prometheus.CounterVec
	IngestConsumedTotal *prometheus.CounterVec
	IngestErrorsTotal   *prometheus.CounterVec

	// global
	EgressTotal           *prometheus.CounterVec
	EgressDeliveredTotal  *prometheus.CounterVec
	EgressErrorsTotal     *prometheus.CounterVec
	EgressNetworkLatency  *prometheus.HistogramVec
	EgressDeliveryLatency *prometheus.HistogramVec

	// per endpoint
	EgressAttemptsTotal          *prometheus.CounterVec
	EgressAttemptsDeliveredTotal *prometheus.CounterVec
	EgressAttemptErrorsTotal     *prometheus.CounterVec
	EgressAttemptNetworkLatency  *prometheus.HistogramVec
	EgressAttemptDeliveryLatency *prometheus.HistogramVec
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

		m.EgressTotal,
		m.EgressDeliveredTotal,
		m.EgressErrorsTotal,
		m.EgressDeliveryLatency,

		m.EgressAttemptsTotal,
		m.EgressAttemptsDeliveredTotal,
		m.EgressAttemptErrorsTotal,
		m.EgressAttemptDeliveryLatency,
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

		EgressTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_events_total",
				Help: "Total number of events sent",
			},
			[]string{projectLabel},
		),
		EgressDeliveredTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_events_success",
				Help: "Total number of events successfully delivered",
			},
			[]string{projectLabel},
		),
		EgressErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_events_error",
				Help: "Total number of errors during event delivery",
			},
			[]string{projectLabel},
		),
		EgressNetworkLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_events_network_latency_seconds",
				Help:    "Distribution of event delivery network latency in seconds",
				Buckets: bucketsDefault,
			},
			[]string{projectLabel},
		),
		EgressDeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_events_delivery_latency_seconds",
				Help:    "Distribution of event delivery latency in seconds",
				Buckets: bucketsDefault,
			},
			[]string{projectLabel},
		),

		EgressAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_delivery_attempts_total",
				Help: "Total number of events sent per endpoint",
			},
			[]string{projectLabel, endpointLabel},
		),
		EgressAttemptsDeliveredTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_delivery_attempts_success",
				Help: "Total number of events successfully delivered per endpoint",
			},
			[]string{projectLabel, endpointLabel},
		),
		EgressAttemptErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "convoy_delivery_attempts_error",
				Help: "Total number of errors during event delivery per endpoint",
			},
			[]string{projectLabel, endpointLabel},
		),
		EgressAttemptNetworkLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_delivery_attempts_network_latency_seconds",
				Help:    "Distribution of event delivery network latency in seconds per endpoint",
				Buckets: bucketsDefault,
			},
			[]string{projectLabel, endpointLabel},
		),
		EgressAttemptDeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "convoy_delivery_attempts_latency_seconds",
				Help:    "Distribution of event delivery latency in seconds per endpoint",
				Buckets: bucketsDefault,
			},
			[]string{projectLabel, endpointLabel},
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

func (m *Metrics) IncrementEgressTotal(pUID string, eUID string) {
	m.EgressTotal.With(prometheus.Labels{projectLabel: pUID}).Inc()
	m.EgressAttemptsTotal.With(prometheus.Labels{projectLabel: pUID, endpointLabel: eUID}).Inc()
}

func (m *Metrics) IncrementEgressDeliveredTotal(pUID string, eUID string) {
	m.EgressDeliveredTotal.With(prometheus.Labels{projectLabel: pUID}).Inc()
	m.EgressAttemptsDeliveredTotal.With(prometheus.Labels{projectLabel: pUID, endpointLabel: eUID}).Inc()
}

func (m *Metrics) IncrementEgressErrorsTotal(pUID string, eUID string) {
	m.EgressErrorsTotal.With(prometheus.Labels{projectLabel: pUID}).Inc()
	m.EgressAttemptErrorsTotal.With(prometheus.Labels{projectLabel: pUID, endpointLabel: eUID}).Inc()
}

func (m *Metrics) ObserveEgressNetworkLatency(pUID string, eID string, elapsedMs int64) {
	m.EgressNetworkLatency.
		With(prometheus.Labels{projectLabel: pUID}).Observe(float64(elapsedMs) / 1000)
	m.EgressAttemptNetworkLatency.
		With(prometheus.Labels{projectLabel: pUID, endpointLabel: eID}).Observe(float64(elapsedMs) / 1000)
}

func (m *Metrics) ObserveEgressDeliveryLatency(pUID string, eUID string, elapsedMs int64) {
	m.EgressDeliveryLatency.
		With(prometheus.Labels{projectLabel: pUID}).Observe(float64(elapsedMs) / 1000)
	m.EgressAttemptDeliveryLatency.
		With(prometheus.Labels{projectLabel: pUID, endpointLabel: eUID}).Observe(float64(elapsedMs) / 1000)
}
