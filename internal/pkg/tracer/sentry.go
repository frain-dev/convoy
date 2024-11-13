package tracer

import (
	"context"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type SentryTracer struct {
	cfg        config.SentryConfiguration
	ShutdownFn func(ctx context.Context) error
}

func NewSentryTracer(cfg config.SentryConfiguration) *SentryTracer {
	return &SentryTracer{cfg: cfg}
}

func (st *SentryTracer) Init(componentName string) error {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              st.cfg.DSN,
		ServerName:       componentName,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Debug:            true,
	})
	if err != nil {
		return err
	}

	// Configure Tracer Provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sentryotel.NewSentrySpanProcessor()),
	)

	// Configure OTel SDK.
	otel.SetTracerProvider(tp)

	// Configure Propagator.
	otel.SetTextMapPropagator(sentryotel.NewSentryPropagator())

	st.ShutdownFn = tp.Shutdown

	return nil
}

func (st *SentryTracer) Type() config.TracerProvider {
	return config.SentryTracerProvider
}
func (st *SentryTracer) Capture(*datastore.Project, string, *net.Response, time.Duration) {

}
func (st *SentryTracer) Shutdown(ctx context.Context) error {
	return st.ShutdownFn(ctx)
}
