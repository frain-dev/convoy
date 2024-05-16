package metrics

import (
	"sync"

	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/hibiken/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var reg *prometheus.Registry
var requestDuration *prometheus.HistogramVec

var re, rd sync.Once

func Reg() *prometheus.Registry {
	re.Do(func() {
		reg = prometheus.NewPedanticRegistry()
	})

	return reg
}

// Reset is only intended for use in tests
func Reset() {
	requestDuration, reg = nil, nil
	re, rd = sync.Once{}, sync.Once{}
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
}

func RequestDuration() *prometheus.HistogramVec {
	rd.Do(func() {
		requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Time (in seconds) spent serving HTTP requests.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status_code"})
	})

	return requestDuration
}

func RegisterQueueMetrics(q queue.Queuer) {
	Reg().MustRegister(
		metrics.NewQueueMetricsCollector(q.(*redisqueue.RedisQueue).Inspector()),
		q.(*redisqueue.RedisQueue),
	)
}
