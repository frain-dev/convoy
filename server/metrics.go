package server

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/hibiken/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "request_duration_seconds",
	Help:    "Time (in seconds) spent serving HTTP requests.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "status_code"})

var Reg *prometheus.Registry = prometheus.NewPedanticRegistry()

func RegisterQueueMetrics(q queue.Queuer, cfg config.Configuration) {

	if cfg.Queue.Type != config.RedisQueueProvider {
		return
	}
	Reg.MustRegister(
		metrics.NewQueueMetricsCollector(q.(*redisqueue.RedisQueue).Inspector()),
	)
}

func RegisterDBMetrics(app *applicationHandler) {
	ctx := context.Background()

	Reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "scheduled",
			Help:      "Number of eventDeliveries in the Scheduled state.",
		},
		func() float64 {
			count, err := app.eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.ScheduledEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status scheduled: %v", err)
			}
			return float64(count)
		},
	))

	Reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "processing",
			Help:      "Number of eventDeliveries in the Processing state.",
		},
		func() float64 {
			count, err := app.eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.ProcessingEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Processing: %v", err)
			}
			return float64(count)
		},
	))

	Reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "retry",
			Help:      "Number of eventDeliveries in the Retry state.",
		},
		func() float64 {
			count, err := app.eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.RetryEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Retry: %v", err)
			}
			return float64(count)
		},
	))

	Reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "eventdelivery",
			Name:      "discarded",
			Help:      "Number of eventDeliveries in the Discarded state.",
		},
		func() float64 {
			count, err := app.eventDeliveryRepo.CountDeliveriesByStatus(ctx, datastore.DiscardedEventStatus, datastore.SearchParams{CreatedAtEnd: time.Now().Unix()})
			if err != nil {
				log.Errorf("Error fetching eventdelivery status Discarded: %v", err)
			}
			return float64(count)
		},
	))
}
