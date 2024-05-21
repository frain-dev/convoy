package redis

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/prometheus/client_golang/prometheus"
)

// Namespace used in fully-qualified metrics names.
const namespace = "convoy"

// Descriptors used by RedisQueue
var (
	eventQueueTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "event_queue_scheduled_total"),
		"Total number of tasks scheduled in the event queue",
		[]string{"status"}, nil,
	)
)

func (rq *RedisQueue) Describe(ch chan<- *prometheus.Desc) {
	if rq == nil {
		return
	}
	prometheus.DescribeByCollect(rq, ch)
}

func (rq *RedisQueue) Collect(ch chan<- prometheus.Metric) {
	if rq == nil {
		return
	}
	cfg, err := config.Get()
	if err != nil {
		return
	}
	if !cfg.Metrics.IsEnabled {
		return
	}
	qinfo, err := rq.inspector.GetQueueInfo(string(convoy.CreateEventQueue))
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(
		eventQueueTotalDesc,
		prometheus.GaugeValue,
		float64(qinfo.Size-qinfo.Completed-qinfo.Archived),
		"scheduled", // not yet in db
	)
}
