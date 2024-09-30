package circuit_breaker

import (
	"context"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

const namespace = "circuit_breaker"

var (
	cachedMetrics *Metrics
	metricsConfig *config.MetricsConfiguration
	lastRun       = time.Now()

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
)

type Metrics struct {
	circuitBreakers []CircuitBreaker
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
	prometheus.DescribeByCollect(cb, ch)
}

func (cb *CircuitBreakerManager) Collect(ch chan<- prometheus.Metric) {
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
		metrics, err = cb.collectMetrics()
		if err != nil {
			log.Errorf("Failed to collect metrics data: %v", err)
			return
		}
		cachedMetrics = metrics
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
	}

	lastRun = now
}
