package tracer

import (
	"context"

	"github.com/frain-dev/convoy/config"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DatadogTracer struct {
	cfg config.DatadogConfiguration
}

func (dt *DatadogTracer) Init(componentName string) (shutdownFn, error) {
	provider := ddotel.NewTracerProvider(
		tracer.WithAgentAddr(dt.cfg.AgentURL),
		tracer.WithService(componentName))

	return func(context.Context) error {
		return provider.Shutdown()
	}, nil
}
