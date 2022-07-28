package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/hibiken/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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
	)
}

func RegisterDBMetrics(eventDeliveryRepo datastore.EventDeliveryRepository) {
	ctx := context.Background()

	Reg().MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "scheduled",
			Help:      "Number of eventDeliveries in the Scheduled state.",
		},
		func() float64 {
			count, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.ScheduledEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status scheduled: %v", err)
			}
			return float64(count)
		},
	))

	Reg().MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "processing",
			Help:      "Number of eventDeliveries in the Processing state.",
		},
		func() float64 {
			count, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.ProcessingEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Processing: %v", err)
			}
			return float64(count)
		},
	))

	Reg().MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "retry",
			Help:      "Number of eventDeliveries in the Retry state.",
		},
		func() float64 {
			count, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.RetryEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Retry: %v", err)
			}
			return float64(count)
		},
	))

	Reg().MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "discarded",
			Help:      "Number of eventDeliveries in the Discarded state.",
		},
		func() float64 {
			count, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.DiscardedEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Discarded: %v", err)
			}
			return float64(count)
		},
	))
}
