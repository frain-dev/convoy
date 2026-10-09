package circuit_breaker

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/frain-dev/convoy/config"
)

const namespace = "circuit_breaker"

var (
	metricsMu     sync.Mutex
	cachedMetrics *Metrics
	metricsConfig *config.MetricsConfiguration
	lastRun       time.Time

	circuitBreakerState = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "state"),
		"The current state of the circuit breaker (0: Closed, 1: Half-Open, 2: Open)",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerRequests = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "requests_total"),
		"Total number of requests processed by the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerFailures = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "failures_total"),
		"Total number of failed requests processed by the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerSuccesses = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "successes_total"),
		"Total number of successful requests processed by the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerFailureRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "failure_rate"),
		"Current failure rate of the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerSuccessRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "success_rate"),
		"Current success rate of the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerConsecutiveFailures = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "consecutive_failures"),
		"Number of consecutive failures for the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerNotificationsSent = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "notifications_sent"),
		"Number of notifications sent by the circuit breaker",
		[]string{"key", "tenant_id"}, nil,
	)
	circuitBreakerSampleLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "sample_latency_seconds"),
		"Latency of the circuit breaker sampling process in seconds",
		nil, nil,
	)
)

type Metrics struct {
	circuitBreakers []CircuitBreaker
	SampleLatency   time.Duration
}

func (cb *CircuitBreakerManager) collectMetrics() (*Metrics, error) {
	metrics := &Metrics{}
	cbs, err := cb.loadCircuitBreakers(context.Background())
	if err != nil {
		return metrics, err
	}

	metrics.circuitBreakers = cbs

	return metrics, nil
}

func (cb *CircuitBreakerManager) Describe(ch chan<- *prometheus.Desc) {
	ch <- circuitBreakerState
	ch <- circuitBreakerRequests
	ch <- circuitBreakerFailures
	ch <- circuitBreakerSuccesses
	ch <- circuitBreakerFailureRate
	ch <- circuitBreakerSuccessRate
	ch <- circuitBreakerConsecutiveFailures
	ch <- circuitBreakerNotificationsSent
	ch <- circuitBreakerSampleLatency
}

func (cb *CircuitBreakerManager) Collect(ch chan<- prometheus.Metric) {
	cfg := cb.loadMetricsConfig()
	if cfg == nil || !cfg.IsEnabled {
		return
	}

	sample := time.Duration(cfg.Prometheus.SampleTime) * time.Second
	now := time.Now()

	metricsMu.Lock()
	if cachedMetrics != nil && !lastRun.IsZero() && now.Before(lastRun.Add(sample)) {
		snapshot := cachedMetrics
		metricsMu.Unlock()
		cb.emitMetrics(ch, snapshot)
		return
	}
	metricsMu.Unlock()

	metrics, err := cb.collectMetrics()
	if err != nil {
		if cb.logger != nil {
			cb.logger.Errorf("Failed to collect metrics data: %v", err)
		}
		metricsMu.Lock()
		snapshot := cachedMetrics
		metricsMu.Unlock()
		if snapshot != nil {
			cb.emitMetrics(ch, snapshot)
		}
		return
	}

	metricsMu.Lock()
	cachedMetrics = metrics
	lastRun = now
	metricsMu.Unlock()
	cb.emitMetrics(ch, metrics)
}

func (cb *CircuitBreakerManager) loadMetricsConfig() *config.MetricsConfiguration {
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

func (cb *CircuitBreakerManager) emitMetrics(ch chan<- prometheus.Metric, metrics *Metrics) {
	if metrics == nil {
		return
	}

	for _, metric := range metrics.circuitBreakers {
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerState,
			prometheus.GaugeValue,
			float64(metric.State),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerRequests,
			prometheus.CounterValue,
			float64(metric.Requests),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerFailures,
			prometheus.CounterValue,
			float64(metric.TotalFailures),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerSuccesses,
			prometheus.CounterValue,
			float64(metric.TotalSuccesses),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerFailureRate,
			prometheus.GaugeValue,
			metric.FailureRate,
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerSuccessRate,
			prometheus.GaugeValue,
			metric.SuccessRate,
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerConsecutiveFailures,
			prometheus.GaugeValue,
			float64(metric.ConsecutiveFailures),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerNotificationsSent,
			prometheus.GaugeValue,
			float64(metric.NotificationsSent),
			metric.Key,
			metric.TenantId,
		)
		ch <- prometheus.MustNewConstMetric(
			circuitBreakerSampleLatency,
			prometheus.GaugeValue,
			metrics.SampleLatency.Seconds(),
		)
	}
}
