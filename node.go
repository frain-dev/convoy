package convoy

import "fmt"

type NodeInfo struct {
	NodeId     string `json:"nodeId"`
	NodeIpAddr string `json:"nodeIpAddr"`
	Port       string `json:"port"`
	QueueName  string `json:"queueName"`
}

/* Information/Metadata about Queue */
type QueueMeta struct {
	QueueName string `json:"queueName"`
	QueueType string `json:"queueType"`
}

var ServiceKey = "service/distributed-worker/leader"
var ServiceName = "distributed-worker"

var EventQueueKey = "eventqueue/meta/data"
var DeadLetterQueue = "deadletter/meta/data"

// ttl in seconds
var TTL = 10
var TTLS = fmt.Sprintf("%ds", TTL)
