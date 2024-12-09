package tracer

import (
	"context"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"go.opentelemetry.io/otel"
	"time"

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

func (dt *DatadogTracer) Capture(project *datastore.Project, targetURL string, resp *net.Response, duration time.Duration) {
	if !dt.Licenser.DatadogTracing() {
		return
	}
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
