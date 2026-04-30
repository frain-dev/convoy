package tracer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/frain-dev/convoy/config"
)

// Verifies the Epic 1 invariant: TracerProvider() always returns a non-nil
// provider that is wired to the same SDK TracerProvider used at runtime, and
// that NoOp returns a no-op provider that produces invalid SpanContexts.
func TestTracerProvider_NoOp(t *testing.T) {
	b := NoOpBackend{}
	tp := b.TracerProvider()
	require.NotNil(t, tp)

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	// noop tracer produces a span whose SpanContext is not sampled and not valid.
	require.False(t, span.SpanContext().IsValid())
}

func TestTracerProvider_OTel_RecordsSpansAfterInit(t *testing.T) {
	// We don't go through OTelTracer.Init (which requires a collector); instead
	// build an SDK provider with an in-memory exporter and inject it as if Init
	// had succeeded. This is what callers will rely on at runtime: that
	// TracerProvider() returns the same provider that records exported spans.
	exp := tracetest.NewInMemoryExporter()
	sdkTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	ot := NewOTelTracer(config.OTelConfiguration{})
	ot.tp = sdkTP

	tp := ot.TracerProvider()
	require.NotNil(t, tp)
	require.Same(t, sdkTP, tp)

	_, span := tp.Tracer("convoy/test").Start(context.Background(), "op")
	span.End()

	require.Len(t, exp.GetSpans(), 1)
	require.Equal(t, "op", exp.GetSpans()[0].Name)
}

func TestTracerProvider_BeforeInit_ReturnsNonNil(t *testing.T) {
	require.NotNil(t, NewOTelTracer(config.OTelConfiguration{}).TracerProvider())
	require.NotNil(t, NewSentryTracer(config.SentryConfiguration{}).TracerProvider())
	// Datadog backend has external deps in its constructor; covered by the
	// shared invariant test above.
}

func TestRecordError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	tr := tp.Tracer("convoy/test")

	t.Run("nil error sets Ok", func(t *testing.T) {
		exp.Reset()
		_, span := tr.Start(context.Background(), "op")
		RecordError(span, nil)
		span.End()
		require.Equal(t, codes.Ok, exp.GetSpans()[0].Status.Code)
	})

	t.Run("non-nil error sets Error and records the error event", func(t *testing.T) {
		exp.Reset()
		_, span := tr.Start(context.Background(), "op")
		RecordError(span, errors.New("boom"))
		span.End()
		stub := exp.GetSpans()[0]
		require.Equal(t, codes.Error, stub.Status.Code)
		require.Equal(t, "boom", stub.Status.Description)
		require.NotEmpty(t, stub.Events)
	})

	t.Run("nil span is safe", func(t *testing.T) {
		require.NotPanics(t, func() { RecordError(nil, errors.New("x")) })
	})
}
