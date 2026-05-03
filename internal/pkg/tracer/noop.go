package tracer

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/frain-dev/convoy/config"
)

// NoOpBackend is a no-operation tracer backend implementation.
type NoOpBackend struct{}

func (NoOpBackend) Init(string) error { return nil }
func (NoOpBackend) Type() config.TracerProvider {
	return ""
}
func (NoOpBackend) TracerProvider() trace.TracerProvider {
	return tracenoop.NewTracerProvider()
}
func (NoOpBackend) Shutdown(context.Context) error {
	return nil
}
