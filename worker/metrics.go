package worker

import (
	"github.com/frain-dev/convoy/config"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func RegisterWorkerMetrics(p Producer, cfg config.Configuration) {

	if p.Queues == nil {
		return
	}

	err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "consumer",
			Name:      "processed",
			Help:      "Number of events processed.",
		},
		func() float64 {
			stats := p.worker.Stats()
			return float64(stats[0].Processed)
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
			stats := p.worker.Stats()
			return float64(stats[0].Fails)
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
			stats := p.worker.Stats()
			return float64(stats[0].Retries)
		},
	))
	if err != nil {
		log.Errorf("Error registering retries: %v", err)
	}
}
