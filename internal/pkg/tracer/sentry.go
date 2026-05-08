package tracer

import (
	"context"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	sentryotel "github.com/getsentry/sentry-go/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/frain-dev/convoy/config"
)

type SentryTracer struct {
	cfg        config.SentryConfiguration
	tp         trace.TracerProvider
	ShutdownFn func(ctx context.Context) error
}

func NewSentryTracer(cfg config.SentryConfiguration) *SentryTracer {
	return &SentryTracer{
		cfg: cfg,
		tp:  tracenoop.NewTracerProvider(),
		ShutdownFn: func(ctx context.Context) error {
			return nil
		},
	}
}

func (st *SentryTracer) Init(componentName string) error {
	// Set default sample rate if not configured
	sampleRate := st.cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = 0.1 // Default to 10% sampling if not specified
	}

	opts := sentry.ClientOptions{
		Dsn:              st.cfg.DSN,
		ServerName:       componentName,
		EnableTracing:    true,
		TracesSampleRate: sampleRate,
		Debug:            st.cfg.Debug,
	}

	if st.cfg.Environment != "" {
		opts.Environment = st.cfg.Environment
	}

	err := sentry.Init(opts)
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

	st.tp = tp
	st.ShutdownFn = tp.Shutdown

	return nil
}

func (st *SentryTracer) Type() config.TracerProvider {
	return config.SentryTracerProvider
}

func (st *SentryTracer) TracerProvider() trace.TracerProvider {
	if st.tp == nil {
		return tracenoop.NewTracerProvider()
	}
	return st.tp
}

func (st *SentryTracer) Shutdown(ctx context.Context) error {
	return st.ShutdownFn(ctx)
}
