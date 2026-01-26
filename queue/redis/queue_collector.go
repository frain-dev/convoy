package redis

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
)

// Namespace used in fully qualified metrics names.
const namespace = "convoy"

// Descriptors used by RedisQueue
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

func (q *RedisQueue) Describe(ch chan<- *prometheus.Desc) {
	if q == nil {
		return
	}
	prometheus.DescribeByCollect(q, ch)
}

func (q *RedisQueue) Collect(ch chan<- prometheus.Metric) {
	if q == nil {
		return
	}

	cfg, err := config.Get()
	if err != nil {
		return
	}

	if !cfg.Metrics.IsEnabled {
		return
	}

	ctx := context.Background()
	namespace := "default"

	stats, err := q.backend.QueueStats(ctx, namespace, string(convoy.CreateEventQueue))
	if err != nil {
		if !strings.Contains(err.Error(), "NOT_FOUND") && !strings.Contains(err.Error(), "does not exist") {
			log.Errorf("an error occurred while fetching queue stats for %s: %+v", convoy.CreateEventQueue, err)
		}
		ch <- prometheus.MustNewConstMetric(
			eventQueueTotalDesc,
			prometheus.GaugeValue,
			0,
			"scheduled",
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			eventQueueTotalDesc,
			prometheus.GaugeValue,
			float64(stats.Pending),
			"scheduled",
		)
	}

	workflowStats, err := q.backend.QueueStats(ctx, namespace, string(convoy.EventWorkflowQueue))
	if err != nil {
		if !strings.Contains(err.Error(), "NOT_FOUND") && !strings.Contains(err.Error(), "does not exist") {
			log.Errorf("an error occurred while fetching queue stats for %s: %+v", convoy.EventWorkflowQueue, err)
		}
		ch <- prometheus.MustNewConstMetric(
			eventQueueMatchSubscriptionsTotalDesc,
			prometheus.GaugeValue,
			0,
			"scheduled",
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			eventQueueMatchSubscriptionsTotalDesc,
			prometheus.GaugeValue,
			float64(workflowStats.Pending),
			"scheduled",
		)
	}
}
