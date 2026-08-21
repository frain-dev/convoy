package postgres

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
)

const namespace = "convoy"

var (
	eventQueueTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_queue_scheduled_total"),
		"Total number of tasks scheduled in the event queue",
		[]string{"status"}, nil,
	)
	eventQueueMatchSubscriptionsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_workflow_queue_match_subscriptions_total"),
		"Total number of tasks scheduled in the workflow queue matching subscriptions",
		[]string{"status"}, nil,
	)
)

func (q *PostgresQueue) Describe(ch chan<- *prometheus.Desc) {
	if q == nil {
		return
	}
	ch <- eventQueueTotalDesc
	ch <- eventQueueMatchSubscriptionsTotalDesc
}

func (q *PostgresQueue) Collect(ch chan<- prometheus.Metric) {
	if q == nil {
		return
	}

	cfg, err := config.Get()
	if err != nil || !cfg.Metrics.IsEnabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	counts, err := q.Counts(ctx)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(eventQueueTotalDesc, prometheus.GaugeValue, 0, "scheduled")
		ch <- prometheus.MustNewConstMetric(eventQueueMatchSubscriptionsTotalDesc, prometheus.GaugeValue, 0, "scheduled")
		return
	}

	byName := make(map[string]QueueCount, len(counts))
	for _, row := range counts {
		byName[row.QueueName] = row
	}

	ch <- prometheus.MustNewConstMetric(
		eventQueueTotalDesc,
		prometheus.GaugeValue,
		float64(scheduled(byName[string(convoy.CreateEventQueue)])),
		"scheduled",
	)
	ch <- prometheus.MustNewConstMetric(
		eventQueueMatchSubscriptionsTotalDesc,
		prometheus.GaugeValue,
		float64(scheduled(byName[string(convoy.EventWorkflowQueue)])),
		"scheduled",
	)
}

func scheduled(c QueueCount) int64 {
	return c.Pending + c.Processing
}
