package tracer

import (
	"github.com/frain-dev/convoy/config"
	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type SentryTracer struct {
	cfg config.SentryConfiguration
}

func (st *SentryTracer) Init(componentName string) error {
	sentry.Init(sentry.ClientOptions{
		Dsn:              st.cfg.DSN,
		ServerName:       componentName,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Debug:            true,
	})

	// Configure Tracer Provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sentryotel.NewSentrySpanProcessor()),
	)

	// Configure OTel SDK.
	otel.SetTracerProvider(tp)

	// Configure Propagator.
	otel.SetTextMapPropagator(sentryotel.NewSentryPropagator())

	return nil
}
