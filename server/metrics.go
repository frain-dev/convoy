package server

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "request_duration_seconds",
	Help:    "Time (in seconds) spent serving HTTP requests.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "status_code"})

func RegisterQueueMetrics(queueName convoy.QueueName, q queue.Queuer, cfg config.Configuration) {

	if cfg.Queue.Type != config.RedisQueueProvider {
		return
	}

	err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "length",
			Help:      "Number of events in the queue.",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Size)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_length: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "latency",
			Help:      "queue latency",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Latency)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_latency: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "latencyhrs",
			Help:      "queue latency in hours",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Latency.Hours())
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_latencyhrs: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "latencymins",
			Help:      "queue latency in minutes",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Latency.Minutes())
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_latencymins: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "latencysecs",
			Help:      "queue latency in seconds",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Latency.Seconds())
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_latencysecs: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "processed",
			Help:      "number of processed items",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Processed)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_processed: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "active",
			Help:      "number of active items",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Active)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_active: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "retry",
			Help:      "number of items in retry state",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Retry)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_retry: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "failed",
			Help:      "number of failed items",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Failed)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_failed: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "scheduled",
			Help:      "number of scheduled items",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.Failed)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_scheduled: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "memoryusage",
			Help:      "queue memory usage",
		},
		func() float64 {
			qInfo, _ := q.(*redisqueue.RedisQueue).Inspector().GetQueueInfo(string(queueName))
			return float64(qInfo.MemoryUsage)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_memoryusage: %v", err)
	}

}

func RegisterDBMetrics(app *applicationHandler) {
	ctx := context.Background()
	err := prometheus.Register(prometheus.NewGaugeFunc(
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
	if err != nil {
		log.Errorf("Error registering eventdelivery Scheduled: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
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
	if err != nil {
		log.Errorf("Error registering eventdelivery Processing: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
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
	if err != nil {
		log.Errorf("Error registering eventdelivery Retry: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
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
	if err != nil {
		log.Errorf("Error registering eventdelivery Discarded: %v", err)
	}
}
