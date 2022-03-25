package server

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	memqueue "github.com/frain-dev/convoy/queue/memqueue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "request_duration_seconds",
	Help:    "Time (in seconds) spent serving HTTP requests.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "status_code"})

func RegisterQueueMetrics(q queue.Queuer, cfg config.Configuration) {
	err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "queue",
			Name:      "length",
			Help:      "Number of events in the queue.",
		},
		func() float64 {
			length, _ := queueLength(q, cfg)
			return float64(length)
		},
	))
	if err != nil {
		log.Errorf("Error registering queue_length: %v", err)
	}

	if cfg.Queue.Type == config.RedisQueueProvider {
		err = prometheus.Register(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Subsystem: "redis_queue",
				Name:      "zset_length",
				Help:      "Number of events in the ZSET.",
			},
			func() float64 {
				bodies, err := q.(*redisqueue.RedisQueue).ZRangebyScore(context.Background(), "-inf", "+inf")
				if err != nil {
					log.Errorf("Error ZSET Length: %v", err)
				}
				return float64(len(bodies))
			},
		))
		if err != nil {
			log.Errorf("Error registering zset_length: %v", err)
		}

		err = prometheus.Register(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Subsystem: "redis_queue",
				Name:      "pending_length",
				Help:      "Number of events in pending.",
			},
			func() float64 {
				pending, err := q.(*redisqueue.RedisQueue).XPending(context.Background())
				if err != nil {
					log.Errorf("Error fetching Pending info: %v", err)
				}
				return float64(pending.Count)
			},
		))
		if err != nil {
			log.Infof("Error registering pending_length: %v", err)
		}
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

func queueLength(q queue.Queuer, cfg config.Configuration) (int, error) {
	switch cfg.Queue.Type {
	case config.RedisQueueProvider:
		n, err := q.(*redisqueue.RedisQueue).Length()
		if err != nil {
			log.Infof("Error getting queue length: %v", err)
		}
		return n, err
	case config.InMemoryQueueProvider:
		n, err := q.(*memqueue.MemQueue).Length()
		if err != nil {
			log.Infof("Error getting queue length: %v", err)
		}
		return n, err
	default:
		return 0, nil
	}
}
