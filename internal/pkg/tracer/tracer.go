package tracer

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"time"

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

// Backend is an abstraction for tracng backend (Datadog, Sentry, ...)
type Backend interface {
	Init(componentName string) error
	Type() config.TracerProvider
	Capture(context.Context, *datastore.Project, string, *net.Response, time.Duration)
	Shutdown(ctx context.Context) error
}

type NoOpBackend struct{}

func (NoOpBackend) Init(string) error { return nil }
func (NoOpBackend) Type() config.TracerProvider {
	return ""
}
func (NoOpBackend) Capture(context.Context, *datastore.Project, string, *net.Response, time.Duration) {

}
func (NoOpBackend) Shutdown(context.Context) error {
	return nil
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
