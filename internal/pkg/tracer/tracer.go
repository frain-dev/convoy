package tracer

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/config"
	"go.opentelemetry.io/otel/trace"
)

var ErrInvalidTracerConfiguration = errors.New("invalid tracer configuration")

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

// Backend is an abstraction for tracng backend (Datadog, Sentry, ...)
type Backend interface {
	Init(componentName string) error
}

type shutdownFn func(context.Context) error

func noopShutdownFn(context.Context) error {
	return nil
}

// Global tracer Init function
func Init(tCfg config.TracerConfiguration) (shutdownFn, error) {
	switch tCfg.Type {
	case config.SentryTracerProvider:
		if tCfg.Sentry == (config.SentryConfiguration{}) {
			return noopShutdownFn, ErrInvalidTracerConfiguration
		}

		st := &SentryTracer{tCfg.Sentry}
		return st.Init("web")

	case config.DatadogTracerProvider:
		if tCfg.Datadog == (config.DatadogConfiguration{}) {
			return noopShutdownFn, ErrInvalidTracerConfiguration
		}

		dt := DatadogTracer{}
		return dt.Init("web")

	case config.OTelTracerProvider:
		if tCfg.OTel == (config.OTelConfiguration{}) {
			return noopShutdownFn, ErrInvalidTracerConfiguration
		}

		ot := OTelTracer{}
		return ot.Init("web")

	case config.ElasticTracerProvider:
		et := ElasticTracer{}
		return et.Init("web")
	}

	return noopShutdownFn, nil
}
