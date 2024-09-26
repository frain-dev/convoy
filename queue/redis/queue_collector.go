package redis

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
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

	qinfo, err := q.inspector.GetQueueInfo(string(convoy.CreateEventQueue))
	if err != nil {
		log.Errorf("an error occurred while fetching queue %+v", err)
		return
	}

	qMSinfo, err := q.inspector.GetQueueInfo(string(convoy.EventWorkflowQueue))
	if err != nil {
		log.Errorf("an error occurred while fetching queue %+v", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		eventQueueTotalDesc,
		prometheus.GaugeValue,
		float64(qinfo.Size-qinfo.Completed-qinfo.Archived),
		"scheduled", // not yet in db
	)

	ch <- prometheus.MustNewConstMetric(
		eventQueueMatchSubscriptionsTotalDesc,
		prometheus.GaugeValue,
		float64(qMSinfo.Size-qMSinfo.Completed-qMSinfo.Archived),
		"scheduled",
	)
}
