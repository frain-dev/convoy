package drain

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

// RouteDescendants lets shared database listeners preserve the accepted
// handler's store. HTTP producers still use normal, revocable admission.
func RouteDescendants(producer queue.Queuer, scope string, executors map[string]queue.Queuer) queue.Queuer {
	return &routedQueue{Queuer: producer, scope: scope, executors: executors}
}

type routedQueue struct {
	queue.Queuer
	scope     string
	executors map[string]queue.Queuer
}

func (q *routedQueue) destination(ctx context.Context) queue.Queuer {
	if admitted, ok := ctx.Value(admissionKey{}).(*admittedWork); ok && admitted.preserveExisting && admitted.gate.Scope == q.scope {
		if executor := q.executors[admitted.gate.StoreID]; executor != nil {
			return executor
		}
	}
	return q.Queuer
}
func (q *routedQueue) Write(ctx context.Context, task convoy.TaskName, name convoy.QueueName, job *queue.Job) error {
	return q.destination(ctx).Write(ctx, task, name, job)
}
func (q *routedQueue) WriteWithoutTimeout(ctx context.Context, task convoy.TaskName, name convoy.QueueName, job *queue.Job) error {
	return q.destination(ctx).WriteWithoutTimeout(ctx, task, name, job)
}
