package metrics

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"sync"

	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/hibiken/asynq/x/metrics"
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

func RegisterQueueMetrics(q queue.Queuer, db database.Database) {
	Reg().MustRegister(metrics.NewQueueMetricsCollector(q.(*redisqueue.RedisQueue).Inspector()))

	configuration, err := config.Get()
	if err == nil && configuration.Metrics.IsEnabled {
		Reg().MustRegister(q.(*redisqueue.RedisQueue), db.(*postgres.Postgres))
	}
}
