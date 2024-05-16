package redis

import (
	"fmt"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
)

// Namespace used in fully-qualified metrics names.
const namespace = "convoy"

// Descriptors used by RedisQueue
var (
	queueSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "queue_size"),
		"Number of tasks in a queue",
		[]string{"source", "queue"}, nil,
	)

	queueLatencyDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "queue_latency_seconds"),
		"Number of seconds the oldest pending task is waiting in pending state to be processed.",
		[]string{"source", "queue"}, nil,
	)
)

func (rq *RedisQueue) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(rq, ch)
}

func (rq *RedisQueue) Collect(ch chan<- prometheus.Metric) {
	queueInfos, err := rq.collectQueueInfo()
	if err != nil {
		log.Printf("Failed to collect metrics data: %v", err)
		return
	}

	for _, info := range queueInfos {

		ch <- prometheus.MustNewConstMetric(
			queueSizeDesc,
			prometheus.GaugeValue,
			float64(info.Size),
			"redis",
			info.Queue,
		)

		ch <- prometheus.MustNewConstMetric(
			queueLatencyDesc,
			prometheus.GaugeValue,
			info.Latency.Seconds(),
			"redis",
			info.Queue,
		)
	}
}

// collectQueueInfo gathers QueueInfo of all queues. (extract from asynq lib)
// Since this operation is expensive, it must be called once per collection.
func (rq *RedisQueue) collectQueueInfo() ([]*asynq.QueueInfo, error) {
	qnames, err := rq.inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue names: %v", err)
	}
	infos := make([]*asynq.QueueInfo, len(qnames))
	for i, qname := range qnames {
		qinfo, err := rq.inspector.GetQueueInfo(qname)
		if err != nil {
			return nil, fmt.Errorf("failed to get queue info: %v", err)
		}
		infos[i] = qinfo
	}
	return infos, nil
}
