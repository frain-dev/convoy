package metrics

import (
	"sync"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"

	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/prometheus/client_golang/prometheus"
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

func RegisterQueueMetrics(q queue.Queuer, db database.Database, cbm *cb.CircuitBreakerManager) {
	configuration, err := config.Get()
	if err == nil && configuration.Metrics.IsEnabled {
		// Only register metrics for Redis queue
		redisQ, ok := q.(*redisqueue.RedisQueue)
		if !ok {
			return
		}

		if cbm == nil {
			Reg().MustRegister(redisQ, db.(*postgres.Postgres))
		} else {
			Reg().MustRegister(redisQ, db.(*postgres.Postgres), cbm)
		}
	}
}
