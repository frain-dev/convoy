package circuit_breaker

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func resetCircuitBreakerCollectorState() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	cachedMetrics = nil
	lastRun = time.Time{}
	metricsConfig = nil
}

func TestCollectServesCachedMetricsWithinSampleWindow(t *testing.T) {
	resetCircuitBreakerCollectorState()
	metricsConfig = &config.MetricsConfiguration{
		IsEnabled: true,
		Prometheus: config.PrometheusMetricsConfiguration{
			SampleTime: 60,
		},
	}
	cachedMetrics = &Metrics{
		circuitBreakers: []CircuitBreaker{{
			Key:      "endpoint-1",
			TenantId: "project-1",
		}},
	}
	lastRun = time.Now()

	cb := &CircuitBreakerManager{logger: log.New("circuit-breaker-collector-test", log.LevelError)}
	ch := make(chan prometheus.Metric, 16)
	cb.Collect(ch)
	close(ch)

	var n int
	for range ch {
		n++
	}
	require.Greater(t, n, 0)
}

func TestDescribeDoesNotCollect(t *testing.T) {
	resetCircuitBreakerCollectorState()
	metricsConfig = &config.MetricsConfiguration{IsEnabled: true}

	cb := &CircuitBreakerManager{logger: log.New("circuit-breaker-collector-test", log.LevelError)}
	ch := make(chan *prometheus.Desc, 16)
	cb.Describe(ch)
	close(ch)

	var n int
	for range ch {
		n++
	}
	require.Equal(t, 9, n)
}
