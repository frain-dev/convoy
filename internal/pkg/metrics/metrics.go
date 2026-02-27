package metrics

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
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

	redisQueue, ok := q.(*redisqueue.RedisQueue)
	if !ok {
		return errors.New("failed to assert redis queue")
	}

	postgresDB, ok := db.(*postgres.Postgres)
	if !ok {
		return errors.New("failed to assert postgres database")
	}

	refreshInterval := time.Duration(configuration.Metrics.Prometheus.SampleTime) * time.Second
	if refreshInterval <= 0 {
		refreshInterval = 30 * time.Second
	}
	postgresDB.ConfigureQueueMetricsSnapshot(rdb, refreshInterval)

	registry := Reg()

	// Register queue and database collectors
	if err := registry.Register(redisQueue); err != nil {
		return fmt.Errorf("failed to register redis queue: %w", err)
	}
	if err := registry.Register(postgresDB); err != nil {
		return fmt.Errorf("failed to register postgres database: %w", err)
	}

	// Register circuit breaker if provided
	if cbm != nil {
		if err := registry.Register(cbm); err != nil {
			return fmt.Errorf("failed to register circuit breaker: %w", err)
		}
	}

	return nil
}
