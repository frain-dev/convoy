package tracer

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
)

var (
	ErrInvalidTracerConfiguration = errors.New("invalid tracer configuration")
	ErrTracerFeatureUnavailable   = errors.New("tracer feature unavailable, please upgrade")

	pkgLogger log.Logger = log.New("tracer", log.LevelInfo)
)

// Backend is the lifecycle abstraction for a tracing backend (OTel, Sentry,
// Datadog, or NoOp). It owns Init/Shutdown and exposes the underlying
// TracerProvider; callers acquire a per-package trace.Tracer from there and
// create hierarchical spans with Start/End.
type Backend interface {
	Init(componentName string) error
	Type() config.TracerProvider

	// TracerProvider returns the underlying trace.TracerProvider used to
	// acquire per-package tracers. Always returns a non-nil provider; before
	// Init has run successfully it returns a no-op provider so callers can
	// hold a tracer without nil-checking.
	TracerProvider() trace.TracerProvider

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

		err := st.Init(componentName)
		return st, err

	case config.DatadogTracerProvider:
		dt := NewDatadogTracer(tCfg.Datadog, licenser)
		if !licenser.DatadogTracing() {
			pkgLogger.Error(ErrTracerFeatureUnavailable.Error())
			return dt, nil
		}
		if tCfg.Datadog == (config.DatadogConfiguration{}) {
			return dt, ErrInvalidTracerConfiguration
		}

		err := dt.Init(componentName)
		return dt, err

	case config.OTelTracerProvider:
		ot := NewOTelTracer(tCfg.OTel)
		if tCfg.OTel == (config.OTelConfiguration{}) {
			return ot, ErrInvalidTracerConfiguration
		}

		err := ot.Init(componentName)
		return ot, err
	}

	return &NoOpBackend{}, nil
}
