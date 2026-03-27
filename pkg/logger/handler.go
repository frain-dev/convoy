package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy"
)

// traceHandler is a slog.Handler that injects OpenTelemetry trace context
// and request ID into every log record when available in the context.
type traceHandler struct {
	inner slog.Handler
}

func newTraceHandler(inner slog.Handler) *traceHandler {
	return &traceHandler{inner: inner}
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	if spanCtx.HasTraceID() {
		r.AddAttrs(slog.String("trace_id", spanCtx.TraceID().String()))
	}
	if spanCtx.HasSpanID() {
		r.AddAttrs(slog.String("span_id", spanCtx.SpanID().String()))
	}

	if reqID, ok := ctx.Value(convoy.RequestIDKey).(string); ok && reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}

	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}
