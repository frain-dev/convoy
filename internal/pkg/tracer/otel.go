package tracer

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

var ErrInvalidCollectorURL = errors.New("invalid OTel Collector URL")
var ErrInvalidOTelSSLConfig = errors.New("invalid OTel ssl cert or key configuration")
var ErrFailedToCreateTLSCredentials = errors.New("failed to create tls credentials from config")

type OTelTracer struct {
	cfg config.OTelConfiguration
}

func (ot *OTelTracer) Init(componentName string) (shutdownFn, error) {
	var opts []otlptracegrpc.Option

	if util.IsStringEmpty(ot.cfg.CollectorURL) {
		return noopShutdownFn, ErrInvalidCollectorURL
	}
	opts = append(opts, otlptracegrpc.WithEndpoint(ot.cfg.CollectorURL))

	if ot.cfg.OTelAuth != (config.OTelAuthConfiguration{}) {
		opts = append(opts, otlptracegrpc.WithHeaders(
			map[string]string{
				ot.cfg.OTelAuth.HeaderName: ot.cfg.OTelAuth.HeaderValue}))
	}

	if ot.cfg.InsecureSkipVerify {
		secureOption := otlptracegrpc.WithInsecure()
		opts = append(opts, secureOption)
	}

	exporter, err := otlptrace.New(context.Background(), otlptracegrpc.NewClient(opts...))
	if err != nil {
		return noopShutdownFn, err
	}

	// Configure Resources.
	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.KeyValue{
				Key:   semconv.ServiceNameKey,
				Value: attribute.StringValue(componentName),
			},
			attribute.KeyValue{
				Key:   semconv.ServiceVersionKey,
				Value: attribute.StringValue(convoy.GetVersion()),
			},
		),
	)
	if err != nil {
		return noopShutdownFn, err
	}

	// Configure Tracer Provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ot.cfg.SampleRate)),
	)

	// Configure OTel SDK
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp.Shutdown, nil
}
