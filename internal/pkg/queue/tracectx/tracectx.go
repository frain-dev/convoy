// Package tracectx carries W3C trace context across the Asynq queue boundary
// using asynq.Task.Headers (added in asynq v0.26.0). The producer populates
// queue.Job.Headers via InjectIntoJob immediately before enqueue; the queue
// driver passes those into asynq.NewTaskWithHeaders, where they ride
// alongside the payload through Redis. The consumer middleware reads
// asynq.Task.Headers and feeds them to ExtractContext so the worker span
// becomes a child of the producer's.
//
// There is no envelope, no payload mutation, and no codec on the wire — task
// payloads are byte-identical to what the caller enqueued.
package tracectx

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/frain-dev/convoy/queue"
)

// InjectIntoJob writes the W3C trace context from ctx into job.Headers so
// the queue producer can hand it to asynq.NewTaskWithHeaders. Safe to call
// when ctx has no active span — Headers stays empty and the consumer just
// starts a root span.
func InjectIntoJob(ctx context.Context, job *queue.Job) {
	if job == nil {
		return
	}
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(job.Headers))
}

// ExtractContext returns a context with the trace context decoded from the
// asynq task's headers. ctx is the consumer's base context (typically the
// asynq handler's). When headers is empty the original ctx is returned, so
// untraced enqueues simply produce root spans on the consumer side.
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}
