package metrics

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics/timeseries"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
)

var reg *prometheus.Registry
var re sync.Once

func Reg() *prometheus.Registry {
	re.Do(func() {
		reg = prometheus.NewPedanticRegistry()
	})

	return reg
}

// Reset is only intended for use in tests
func Reset() {
	reg = nil
	re = sync.Once{}
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
}

func RegisterQueueMetrics(q queue.Queuer, db database.Database, rdb redis.UniversalClient, cbm *cb.CircuitBreakerManager) error {
	configuration, err := config.Get()
	if err != nil || !configuration.Metrics.IsEnabled {
		return err
	}

	registry := Reg()

	// Register redis queue collector
	redisQueue, ok := q.(*redisqueue.RedisQueue)
	if ok {
		if err := registry.Register(redisQueue); err != nil {
			return fmt.Errorf("failed to register redis queue: %w", err)
		}
	}

	// Register metrics collector based on backend configuration
	switch configuration.Metrics.Backend {
	case config.TimeSeriesMetricsProvider:
		if rdb == nil {
			return errors.New("redis client is required for timeseries metrics backend")
		}
		tsCollector := timeseries.NewRedisTimeSeriesCollector(rdb, configuration.Metrics.TimeSeries)
		if err := registry.Register(tsCollector); err != nil {
			return fmt.Errorf("failed to register timeseries collector: %w", err)
		}
	case config.PrometheusMetricsProvider, "":
		// Legacy Postgres collector
		postgresDB, ok := db.(*postgres.Postgres)
		if ok {
			if err := registry.Register(postgresDB); err != nil {
				return fmt.Errorf("failed to register postgres database: %w", err)
			}
		}
	default:
		return fmt.Errorf("unknown metrics backend: %s", configuration.Metrics.Backend)
	}

	// Register circuit breaker if provided
	if cbm != nil {
		if err := registry.Register(cbm); err != nil {
			return fmt.Errorf("failed to register circuit breaker: %w", err)
		}
	}

	return nil
}
