package tracectx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/frain-dev/convoy/queue"
)

// End-to-end without the asynq queue: a span on the producer side ends up as
// the parent of a span started on the consumer side after the trace context
// round-trips through queue.Job.Headers. This is the contract the production
// code relies on when it calls asynq.NewTaskWithHeaders(payload, job.Headers)
// on one side and asynq.Task.Headers() on the other.
func TestInjectIntoJob_ProducesChildSpanOnExtract(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	// Save and restore the global TP/propagator so we don't pollute sibling
	// tests in the same binary. `make test` runs the whole module under
	// -race -p 1; without restoration, anything that calls otel.Tracer(...)
	// after this test would emit spans into our local exp.
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	producerCtx, producerSpan := tp.Tracer("producer").Start(context.Background(), "produce")
	job := &queue.Job{Payload: []byte(`{"x":1}`)}
	InjectIntoJob(producerCtx, job)
	producerSpan.End()

	require.NotEmpty(t, job.Headers, "InjectIntoJob must populate Headers when there is an active span")
	require.Contains(t, job.Headers, "traceparent", "expected W3C traceparent header")

	// Consumer side: extract context from the same headers (which is what
	// the worker middleware now reads off asynq.Task.Headers()) and start
	// a child span. It must share the producer's trace ID.
	consumerCtx := ExtractContext(context.Background(), job.Headers)
	_, consumerSpan := tp.Tracer("consumer").Start(consumerCtx, "consume")
	consumerSpan.End()

	require.NoError(t, tp.ForceFlush(t.Context()))
	spans := exp.GetSpans()
	require.Len(t, spans, 2)
	require.Equal(t, spans[0].SpanContext.TraceID(), spans[1].SpanContext.TraceID(),
		"consumer span must share trace ID with producer when headers carry the context")
}

// InjectIntoJob with no active span leaves headers empty so the consumer
// starts a fresh root span. ExtractContext on empty headers is a no-op.
func TestInjectIntoJob_NoActiveSpan(t *testing.T) {
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	job := &queue.Job{Payload: []byte("raw")}
	InjectIntoJob(context.Background(), job)
	require.Empty(t, job.Headers, "no active span → no traceparent injected")

	// Round-trip with empty headers must be a no-op on the context.
	got := ExtractContext(context.Background(), job.Headers)
	require.Equal(t, context.Background(), got)
}

func TestInjectIntoJob_NilJobIsSafe(t *testing.T) {
	require.NotPanics(t, func() {
		InjectIntoJob(context.Background(), nil)
	})
}
