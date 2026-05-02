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

// Empty/nil headers must skip the envelope entirely so non-traced callers
// pay no encoding overhead. Unwrap then treats the bytes as a legacy payload.
func TestWrap_EmptyHeadersPassesThroughRaw(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	require.Equal(t, payload, Wrap(nil, payload))
	require.Equal(t, payload, Wrap(map[string]string{}, payload))

	// Round-trips through Unwrap as a raw payload (no headers).
	got, headers := Unwrap(Wrap(nil, payload))
	require.Equal(t, payload, got)
	require.Nil(t, headers)
}

func TestWrap_RoundtripPreservesPayload(t *testing.T) {
	original := []byte(`{"hello":"world"}`)
	wrapped := Wrap(map[string]string{"traceparent": "abc"}, original)
	require.NotEqual(t, original, wrapped, "wrapped bytes must differ from raw payload")
	require.Equal(t, envelopeMagic, wrapped[0])

	got, headers := Unwrap(wrapped)
	require.Equal(t, original, got)
	require.Equal(t, "abc", headers["traceparent"])
}

func TestUnwrap_LegacyPayloadPassesThrough(t *testing.T) {
	// A payload enqueued before the envelope existed must keep flowing through.
	legacy := []byte(`{"hello":"world"}`) // starts with '{'
	got, headers := Unwrap(legacy)
	require.Equal(t, legacy, got)
	require.Nil(t, headers)
}

func TestUnwrap_MalformedEnvelopeFallsBackToRaw(t *testing.T) {
	// Magic byte but invalid JSON body — fall back rather than crash.
	bad := append([]byte{envelopeMagic}, []byte("not-json")...)
	got, headers := Unwrap(bad)
	require.Equal(t, bad, got)
	require.Nil(t, headers)
}

func TestUnwrap_EmptyBytesAreSafe(t *testing.T) {
	got, headers := Unwrap(nil)
	require.Nil(t, got)
	require.Nil(t, headers)
}

// End-to-end: a span on the producer side ends up as the parent of a span
// started on the consumer side after the payload survives an enqueue/dequeue.
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

	wrapped := Wrap(job.Headers, job.Payload)

	// On the consumer side: unwrap, extract context, start a span — its parent
	// trace ID should match the producer's.
	payload, headers := Unwrap(wrapped)
	require.Equal(t, job.Payload, payload)
	require.NotEmpty(t, headers)

	consumerCtx := ExtractContext(context.Background(), headers)
	_, consumerSpan := tp.Tracer("consumer").Start(consumerCtx, "consume")
	consumerSpan.End()

	require.NoError(t, tp.ForceFlush(t.Context()))
	spans := exp.GetSpans()
	require.Len(t, spans, 2)
	// Both spans must share the same trace ID.
	require.Equal(t, spans[0].SpanContext.TraceID(), spans[1].SpanContext.TraceID())
}
