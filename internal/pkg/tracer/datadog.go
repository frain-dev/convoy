package tracer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DatadogTracer struct {
	cfg          config.DatadogConfiguration
	StatsdClient *statsd.Client
	Licenser     license.Licenser
	ShutdownFn   func(ctx context.Context) error
}

func NewDatadogTracer(cfg config.DatadogConfiguration, licenser license.Licenser) *DatadogTracer {
	return &DatadogTracer{
		cfg:      cfg,
		Licenser: licenser,
		ShutdownFn: func(ctx context.Context) error {
			return nil
		},
	}
}

func (dt *DatadogTracer) Init(componentName string) error {
	provider := ddotel.NewTracerProvider(
		tracer.WithLogStartup(false),
		tracer.WithAgentAddr(dt.cfg.AgentURL),
		tracer.WithService(componentName))

	statsdClient, err := statsd.New(dt.cfg.AgentURL)
	if err != nil {
		log.Fatal(err)
	}
	dt.StatsdClient = statsdClient

	// Configure OTel SDK.
	otel.SetTracerProvider(provider)

	dt.ShutdownFn = func(context.Context) error {
		defer dt.StatsdClient.Close()
		return provider.Shutdown()
	}

	return nil
}

func (dt *DatadogTracer) Type() config.TracerProvider {
	return config.DatadogTracerProvider
}

func (dt *DatadogTracer) CaptureDelivery(ctx context.Context, project *datastore.Project, targetURL string, resp *net.Response, duration time.Duration) {
	if !dt.Licenser.DatadogTracing() {
		return
	}

	traceId := getDatadogTraceID(ctx)
	fmt.Printf("%s\n", traceId)

	var status string
	var statusCode int
	if resp != nil {
		status = resp.Status
		statusCode = resp.StatusCode
	}
	dt.RecordLatency(project.UID, targetURL, status, duration)
	if resp != nil {
		dt.RecordThroughput(project.UID, targetURL, len(resp.Body))
	}
	dt.RecordRequestTotal(project.UID, targetURL)
	if statusCode > 299 {
		dt.RecordErrorRate(project.UID, targetURL, statusCode)
	}
}

func (dt *DatadogTracer) Shutdown(ctx context.Context) error {
	return dt.ShutdownFn(ctx)
}

func (dt *DatadogTracer) RecordLatency(projectID string, url string, status string, duration time.Duration) {
	tags := []string{"project:" + projectID, "url:" + url, "status:" + status}
	err := dt.StatsdClient.Timing("convoy.request.latency.avg", duration, tags, 1)
	if err != nil {
		log.Errorf("Error recording latency: %s", err)
	}
	err = dt.StatsdClient.Histogram("convoy.request.latency.95percentile", float64(duration.Milliseconds()), tags, 1)
	if err != nil {
		log.Errorf("Error recording latency 95percentile: %s", err)
	}
}

func (dt *DatadogTracer) RecordErrorRate(projectID string, url string, statusCode int) {
	tags := []string{"project:" + projectID, "url:" + url}
	if statusCode >= 400 && statusCode < 500 {
		err := dt.StatsdClient.Incr("convoy.request.errors.4xx", tags, 1)
		if err != nil {
			log.Errorf("Error recording 4xx error rate: %s", err)
		}
	} else if statusCode >= 500 {
		err := dt.StatsdClient.Incr("convoy.request.errors.5xx", tags, 1)
		if err != nil {
			log.Errorf("Error recording 5xx error rate: %s", err)
		}
	}
}

func (dt *DatadogTracer) RecordRequestTotal(projectID string, url string) {
	tags := []string{"project:" + projectID, "url:" + url}
	err := dt.StatsdClient.Incr("convoy.request.total", tags, 1)
	if err != nil {
		log.Errorf("Error recording request total: %s", err)
	}
}

func (dt *DatadogTracer) RecordThroughput(projectID string, url string, dataSizeBytes int) {
	tags := []string{"project:" + projectID, "url:" + url}
	dataSizeMB := float64(dataSizeBytes) / (1024 * 1024)
	err := dt.StatsdClient.Gauge("convoy.data.throughput", dataSizeMB, tags, 1)
	if err != nil {
		log.Errorf("Error recording throughput: %s", err)
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

func (dt *DatadogTracer) Capture(ctx context.Context, name string, attributes map[string]interface{}, startTime time.Time, endTime time.Time) {
	if !dt.Licenser.DatadogTracing() {
		return
	}

	_, span := otel.Tracer("").Start(ctx, name, trace.WithTimestamp(startTime))
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
