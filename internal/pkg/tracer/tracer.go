package tracer

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrInvalidTracerConfiguration = errors.New("invalid tracer configuration")
	ErrTracerFeatureUnavailable   = errors.New("tracer feature unavailable, please upgrade")
)

type contextKey string

const (
	TracingContextKey contextKey = "tracerCtx"
)

func NewContext(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, TracingContextKey, tracer)
}

func FromContext(ctx context.Context) trace.Tracer {
	v := ctx.Value(TracingContextKey)
	if v == nil {
		panic("nil context")
	}

	// TODO(subomi): Figure out what to do here or use a tracing struct
	return nil
}

// Backend is an abstraction for tracing backend (Datadog, Sentry, ...)
type Backend interface {
	Init(componentName string) error
	Type() config.TracerProvider
	Capture(ctx context.Context, name string, attributes map[string]interface{}, startTime time.Time, endTime time.Time)
	Shutdown(ctx context.Context) error
}

// Init is a global tracer initialization function
func Init(tCfg config.TracerConfiguration, componentName string, licenser license.Licenser) (Backend, error) {
	switch tCfg.Type {
	case config.SentryTracerProvider:
		st := NewSentryTracer(tCfg.Sentry)
		if tCfg.Sentry == (config.SentryConfiguration{}) {
			return st, ErrInvalidTracerConfiguration
		}

		return st, st.Init(componentName)

	case config.DatadogTracerProvider:
		dt := NewDatadogTracer(tCfg.Datadog, licenser)
		if !licenser.DatadogTracing() {
			log.Error(ErrTracerFeatureUnavailable.Error())
			return dt, nil
		}
		if tCfg.Datadog == (config.DatadogConfiguration{}) {
			return dt, ErrInvalidTracerConfiguration
		}

		return dt, dt.Init(componentName)

	case config.OTelTracerProvider:
		ot := NewOTelTracer(tCfg.OTel)
		if tCfg.OTel == (config.OTelConfiguration{}) {
			return ot, ErrInvalidTracerConfiguration
		}

		return ot, ot.Init(componentName)
	}

	return &NoOpBackend{}, nil
}

func getTraceID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		traceID := spanCtx.TraceID()
		return traceID.String()
	}
	return ""
}
