package tracer

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/util"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/credentials"

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
	cfg        config.OTelConfiguration
	ShutdownFn func(ctx context.Context) error
}

func NewOTelTracer(cfg config.OTelConfiguration) *OTelTracer {
	return &OTelTracer{
		cfg: cfg,
		ShutdownFn: func(ctx context.Context) error {
			return nil
		},
	}
}

func (ot *OTelTracer) Init(componentName string) error {
	var opts []otlptracegrpc.Option

	if util.IsStringEmpty(ot.cfg.CollectorURL) {
		return ErrInvalidCollectorURL
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
	} else {
		secureOption := otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
		opts = append(opts, secureOption)
	}

	exporter, err := otlptrace.New(context.Background(), otlptracegrpc.NewClient(opts...))
	if err != nil {
		return err
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
		return err
	}

	// Configure Tracer Provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ot.cfg.SampleRate)),
	)

	// Configure OTel SDK
	otel.SetTracerProvider(tp)

	// Configure Propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	ot.ShutdownFn = tp.Shutdown

	return nil
}

func (ot *OTelTracer) Type() config.TracerProvider {
	return config.OTelTracerProvider
}

func (ot *OTelTracer) Capture(ctx context.Context, project *datastore.Project, targetURL string, resp *net.Response, duration time.Duration) {
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

func (ot *OTelTracer) Shutdown(ctx context.Context) error {
	return ot.ShutdownFn(ctx)
}
