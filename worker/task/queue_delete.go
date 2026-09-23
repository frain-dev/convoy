package task

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

type jobRemover interface {
	DeleteEventDeliveriesFromQueue(queueName convoy.QueueName, ids []string) error
}

type admittedJobRemover interface {
	DeleteEventDeliveriesFromQueueContext(ctx context.Context, queueName convoy.QueueName, ids []string) error
}

// removeQueuedJobs drops existing broker rows before a re-enqueue. Redis
// cancels in-flight asynq tasks then deletes. Postgres deletes pending and
// archived rows only; processing stays claimed so the worker is not double-run.
func removeQueuedJobs(ctx context.Context, q queue.Queuer, queueName convoy.QueueName, ids []string) error {
	if r, ok := q.(admittedJobRemover); ok {
		return r.DeleteEventDeliveriesFromQueueContext(ctx, queueName, ids)
	}
	r, ok := q.(jobRemover)
	if !ok {
		return nil
	}
	return r.DeleteEventDeliveriesFromQueue(queueName, ids)
}
