package apm

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

// openTelemetryAPM is an implementation of the APM interface for OpenTelemetry.
type openTelemetryAPM struct {
	tracer trace.Tracer
}

// NewOpenTelemetryAPM initializes a new OpenTelemetry APM instance.
func NewOpenTelemetryAPM(serviceName string) (APM, error) {
	exporter, err := otlptrace.New(context.Background(), otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}

	resources, err := resource.New(context.Background())
	if err != nil {
		return nil, err
	}

	// Setup OpenTelemetry SDK and TracerProvider here...
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
			sdktrace.WithSyncer(exporter),
			sdktrace.WithResource(resources),
		),
	)

	tracer := otel.Tracer(serviceName)
	return &openTelemetryAPM{tracer: tracer}, nil
}

func (ot *openTelemetryAPM) StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (Transaction, *http.Request, http.ResponseWriter) {
	ctx, span := ot.tracer.Start(context.Background(), name)
	return &openTelemetryTransaction{ctx: ctx, span: span}, r, w
}

func (ot *openTelemetryAPM) StartTransaction(ctx context.Context, name string) (Transaction, context.Context) {
	ctxt, span := ot.tracer.Start(ctx, name)
	return &openTelemetryTransaction{ctx: ctxt, span: span}, ctxt
}

func (ot *openTelemetryAPM) Shutdown() {
	// Add logic to shut down TracerProvider if necessary.
}

// openTelemetryTransaction is an implementation of the Transaction interface for OpenTelemetry.
type openTelemetryTransaction struct {
	ctx  context.Context
	span trace.Span
}

func (ott *openTelemetryTransaction) AddTag(key string, value interface{}) {
	ott.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
}

func (ott *openTelemetryTransaction) End() {
	ott.span.End()
}

func (ott *openTelemetryTransaction) RecordError(err error) {
	// Set the status of the span to reflect an error has occurred.
	// This is typically done by setting the status code to Error.
	ott.span.SetStatus(codes.Error, err.Error())

	// Record the error as an event in the span. You can add additional attributes as needed.
	ott.span.RecordError(err, trace.WithAttributes(
		// Here you can add attributes to provide more details about the error.
		// For example, you can add the type of the error, stack trace, etc.
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", err.Error()),
		// Add more attributes if needed.
	))
}

//func (ott *openTelemetryTransaction) StartSegment(name string) Segment {
//	_, span := otel.Tracer("").Start(ott.ctx, name)
//	return &openTelemetrySegment{span: span}
//}0-p
//
//// openTelemetrySegment is an implementation of the Segment interface for OpenTelemetry.
//type openTelemetrySegment struct {
//	span trace.Span
//}
//
//func (ots *openTelemetrySegment) End() {
//	ots.span.End()
//}
