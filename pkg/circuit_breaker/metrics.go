package circuit_breaker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "The current state of the circuit breaker (0: Closed, 1: Half-Open, 2: Open)",
		},
		[]string{"key"},
	)

	circuitBreakerRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "circuit_breaker_requests_total",
			Help: "The total number of requests processed by the circuit breaker",
		},
		[]string{"key", "result"},
	)
)

func (cb *CircuitBreakerManager) UpdateMetrics(breaker CircuitBreaker) {
	// todo(raymond) call UpdateMetrics in the sampleStore method after updating each circuit breaker
	circuitBreakerState.WithLabelValues(breaker.Key).Set(float64(breaker.State))
	circuitBreakerRequests.WithLabelValues(breaker.Key, "success").Add(float64(breaker.TotalSuccesses))
	circuitBreakerRequests.WithLabelValues(breaker.Key, "failure").Add(float64(breaker.TotalFailures))
}
