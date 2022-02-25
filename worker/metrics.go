package worker

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func RegisterWorkerMetrics(q queue.Queuer, cfg config.Configuration) {

	if q.Consumer() == nil {
		return
	}

	err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "num_workers",
			Help:      "Number of workers.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.NumWorker)
		},
	))
	if err != nil {
		log.Errorf("Metrics: Error registering num_workers %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "num_fetchers",
			Help:      "Number of fetchers.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.NumFetcher)
		},
	))
	if err != nil {
		log.Errorf("Metrics: Error registering num_fetchers %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "buffers_size",
			Help:      "Buffer size.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.BufferSize)
		},
	))
	if err != nil {
		log.Errorf("Metrics: Error registering buffer_size %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "buffered",
			Help:      "Number of events buffered.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.Buffered)
		},
	))
	if err != nil {
		log.Errorf("Metrics: Error registering buffered %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "processed",
			Help:      "Number of events processed.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.Processed)
		},
	))
	if err != nil {
		log.Errorf("Error registering processed: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "fails",
			Help:      "Number of fails.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.Fails)
		},
	))
	if err != nil {
		log.Errorf("Error registering fails: %v", err)
	}

	err = prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "retries",
			Help:      "Number of retries.",
		},
		func() float64 {
			stats := q.Consumer().Stats()
			return float64(stats.Retries)
		},
	))
	if err != nil {
		log.Errorf("Error registering retries: %v", err)
	}
}
