package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

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

func RegisterQueueMetrics(q queue.Queuer, db database.Database, cbm *cb.CircuitBreakerManager, component string) {
	configuration, err := config.Get()
	if err == nil && configuration.Metrics.IsEnabled {
		if redisQueue, ok := q.(*redisqueue.RedisQueue); ok {
			redisQueue.SetComponent(component)
		}
		if postgresDB, ok := db.(*postgres.Postgres); ok {
			postgresDB.SetComponent(component)
		}

		if cbm == nil { // cbm can be nil if the feature flag is not enabled
			Reg().MustRegister(q.(*redisqueue.RedisQueue), db.(*postgres.Postgres))
		} else {
			Reg().MustRegister(q.(*redisqueue.RedisQueue), db.(*postgres.Postgres), cbm)
		}
	}
}
