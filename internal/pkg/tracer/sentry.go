package tracer

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type SentryTracer struct {
	cfg        config.SentryConfiguration
	ShutdownFn func(ctx context.Context) error
}

func NewSentryTracer(cfg config.SentryConfiguration) *SentryTracer {
	return &SentryTracer{
		cfg: cfg,
		ShutdownFn: func(ctx context.Context) error {
			return nil
		},
	}
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

func (st *SentryTracer) Capture(ctx context.Context, project *datastore.Project, targetURL string, resp *net.Response, duration time.Duration) {
	// Create a new span using the global tracer provider
	_, span := otel.Tracer("").Start(ctx, "webhook_delivery",
		trace.WithTimestamp(time.Now().Add(-duration)))
	defer span.End(trace.WithTimestamp(time.Now()))

	// Add project and URL attributes
	span.SetAttributes(
		attribute.String("project.id", project.UID),
		attribute.String("target.url", targetURL),
	)

	// Add response data if available
	if resp != nil {
		span.SetAttributes(
			attribute.String("response.status", resp.Status),
			attribute.Int("response.status_code", resp.StatusCode),
			attribute.Int("response.size_bytes", len(resp.Body)),
		)
	}

	// Record the duration
	span.SetAttributes(
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
}

func (st *SentryTracer) Shutdown(ctx context.Context) error {
	return st.ShutdownFn(ctx)
}
