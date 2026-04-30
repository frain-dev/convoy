package tracer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	tracenoop "go.opentelemetry.io/otel/trace/noop"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type DatadogTracer struct {
	cfg          config.DatadogConfiguration
	StatsdClient *statsd.Client
	Licenser     license.Licenser
	tp           trace.TracerProvider
	ShutdownFn   func(ctx context.Context) error
	logger       log.Logger
}

func NewDatadogTracer(cfg config.DatadogConfiguration, licenser license.Licenser) *DatadogTracer {
	return &DatadogTracer{
		cfg:      cfg,
		Licenser: licenser,
		tp:       tracenoop.NewTracerProvider(),
		ShutdownFn: func(ctx context.Context) error {
			return nil
		},
		logger: log.New("datadog", log.LevelInfo),
	}
}

func (dt *DatadogTracer) Init(componentName string) error {
	provider := ddotel.NewTracerProvider(
		tracer.WithLogStartup(false),
		tracer.WithAgentAddr(dt.cfg.AgentURL),
		tracer.WithService(componentName))

	statsdClient, err := statsd.New(dt.cfg.AgentURL)
	if err != nil {
		dt.logger.Error(err.Error())
	}
	dt.StatsdClient = statsdClient

	// Configure OTel SDK.
	otel.SetTracerProvider(provider)

	dt.tp = provider
	dt.ShutdownFn = func(context.Context) error {
		defer dt.StatsdClient.Close()
		return provider.Shutdown()
	}

	return nil
}

func (dt *DatadogTracer) Type() config.TracerProvider {
	return config.DatadogTracerProvider
}

func (dt *DatadogTracer) TracerProvider() trace.TracerProvider {
	if dt.tp == nil {
		return tracenoop.NewTracerProvider()
	}
	return dt.tp
}

func (dt *DatadogTracer) CaptureDelivery(ctx context.Context, project *datastore.Project, targetURL, status string, statusCode, bodyLength int, duration time.Duration) {
	if !dt.Licenser.DatadogTracing() {
		return
	}

	traceId := getDatadogTraceID(ctx)
	fmt.Printf("%s\n", traceId)

	dt.RecordLatency(project.UID, targetURL, status, duration)
	dt.RecordThroughput(project.UID, targetURL, bodyLength)
	dt.RecordRequestTotal(project.UID, targetURL)
	if statusCode > 299 {
		dt.RecordErrorRate(project.UID, targetURL, statusCode)
	}
}

func (dt *DatadogTracer) Shutdown(ctx context.Context) error {
	return dt.ShutdownFn(ctx)
}

func (dt *DatadogTracer) RecordLatency(projectID, url, status string, duration time.Duration) {
	tags := []string{"project:" + projectID, "url:" + url, "status:" + status}
	err := dt.StatsdClient.Timing("convoy.request.latency.avg", duration, tags, 1)
	if err != nil {
		dt.logger.Error(fmt.Sprintf("Error recording latency: %s", err))
	}
	err = dt.StatsdClient.Histogram("convoy.request.latency.95percentile", float64(duration.Milliseconds()), tags, 1)
	if err != nil {
		dt.logger.Error(fmt.Sprintf("Error recording latency 95percentile: %s", err))
	}
}

func (dt *DatadogTracer) RecordErrorRate(projectID, url string, statusCode int) {
	tags := []string{"project:" + projectID, "url:" + url}
	if statusCode >= 400 && statusCode < 500 {
		err := dt.StatsdClient.Incr("convoy.request.errors.4xx", tags, 1)
		if err != nil {
			dt.logger.Error(fmt.Sprintf("Error recording 4xx error rate: %s", err))
		}
	} else if statusCode >= 500 {
		err := dt.StatsdClient.Incr("convoy.request.errors.5xx", tags, 1)
		if err != nil {
			dt.logger.Error(fmt.Sprintf("Error recording 5xx error rate: %s", err))
		}
	}
}

func (dt *DatadogTracer) RecordRequestTotal(projectID, url string) {
	tags := []string{"project:" + projectID, "url:" + url}
	err := dt.StatsdClient.Incr("convoy.request.total", tags, 1)
	if err != nil {
		dt.logger.Error(fmt.Sprintf("Error recording request total: %s", err))
	}
}

func (dt *DatadogTracer) RecordThroughput(projectID, url string, dataSizeBytes int) {
	tags := []string{"project:" + projectID, "url:" + url}
	dataSizeMB := float64(dataSizeBytes) / (1024 * 1024)
	err := dt.StatsdClient.Gauge("convoy.data.throughput", dataSizeMB, tags, 1)
	if err != nil {
		dt.logger.Error(fmt.Sprintf("Error recording throughput: %s", err))
	}
}

func getDatadogTraceID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		// Since datadog trace provider (ddtrace) uses big endian uint64 for the trace ID, we must first to first convert it back to uint64.
		traceID := spanCtx.TraceID()
		traceIDRaw := [16]byte(traceID)
		traceIDUint64 := byteArrToUint64(traceIDRaw[8:])
		traceIDStr := strconv.FormatUint(traceIDUint64, 10)
		return traceIDStr
	}
	return ""
}

func byteArrToUint64(buf []byte) uint64 {
	var x uint64
	for i, b := range buf {
		x = x<<8 + uint64(b)
		if i == 7 {
			return x
		}
	}
	return x
}
