package task

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

type jobRemover interface {
	DeleteEventDeliveriesFromQueue(queueName convoy.QueueName, ids []string) error
}

// removeQueuedJobs drops existing broker rows before a re-enqueue. Redis
// cancels in-flight asynq tasks then deletes. Postgres deletes pending and
// archived rows only; processing stays claimed so the worker is not double-run.
func removeQueuedJobs(q queue.Queuer, queueName convoy.QueueName, ids []string) error {
	r, ok := q.(jobRemover)
	if !ok {
		return nil
	}
	return r.DeleteEventDeliveriesFromQueue(queueName, ids)
}
