package tracer

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
)

// NoOpBackend is a no-operation tracer backend implementation.
type NoOpBackend struct{}

func (NoOpBackend) Init(string) error { return nil }
func (NoOpBackend) Type() config.TracerProvider {
	return ""
}
func (NoOpBackend) Capture(context.Context, string, map[string]interface{}, time.Time, time.Time) {}
func (NoOpBackend) Shutdown(context.Context) error {
	return nil
}
