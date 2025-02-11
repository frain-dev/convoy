package tracer

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
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

func (st *SentryTracer) Capture(ctx context.Context, name string, attributes map[string]interface{}, startTime time.Time, endTime time.Time) {
	_, span := otel.Tracer("").Start(ctx, name,
		trace.WithTimestamp(startTime))

	// End span with provided end time
	defer span.End(trace.WithTimestamp(endTime))

	// Convert and set attributes
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, val))
		case int:
			attrs = append(attrs, attribute.Int(k, val))
		case int64:
			attrs = append(attrs, attribute.Int64(k, val))
		case float64:
			attrs = append(attrs, attribute.Float64(k, val))
		case bool:
			attrs = append(attrs, attribute.Bool(k, val))
		}
	}
	span.SetAttributes(attrs...)
}

func (st *SentryTracer) Shutdown(ctx context.Context) error {
	return st.ShutdownFn(ctx)
}
