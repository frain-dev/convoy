package tracer

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

func (st *SentryTracer) Capture(project *datastore.Project, targetURL string, resp *net.Response, duration time.Duration) {
	// Create a transaction
	transaction := sentry.StartTransaction(
		context.Background(),
		"webhook_delivery",
		sentry.WithTransactionName(targetURL),
	)
	defer transaction.Finish()

	// Add project context
	transaction.SetTag("project_id", project.UID)
	transaction.SetTag("target_url", targetURL)

	// Add response data if available
	if resp != nil {
		transaction.SetTag("status", resp.Status)
		transaction.SetTag("status_code", string(rune(resp.StatusCode)))
		transaction.SetData("response_size_bytes", fmt.Sprintf("%d", len(resp.Body)))
	}

	// Record the duration
	transaction.SetData("duration_ms", fmt.Sprintf("%d", duration.Milliseconds()))
}

func (st *SentryTracer) Shutdown(ctx context.Context) error {
	return st.ShutdownFn(ctx)
}
