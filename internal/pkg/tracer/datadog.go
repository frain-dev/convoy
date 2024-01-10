package tracer

import (
	"github.com/frain-dev/convoy/config"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DatadogTracer struct {
	cfg config.DatadogConfiguration
}

func (dt *DatadogTracer) Init(componentName string) error {
	provider := ddotel.NewTracerProvider(
		tracer.WithAgentAddr(dt.cfg.AgentURL),
		tracer.WithService(componentName))

	defer provider.Shutdown()
	return nil
}
