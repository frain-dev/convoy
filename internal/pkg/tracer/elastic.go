package tracer

import (
	"go.elastic.co/apm/module/apmotel/v2"
	"go.opentelemetry.io/otel"
)

type ElasticTracer struct{}

func (et *ElasticTracer) Init(componentName string) (shutdownFn, error) {
	provider, err := apmotel.NewTracerProvider()
	if err != nil {
		return noopShutdownFn, err
	}

	// Configure Tracer Provider.
	otel.SetTracerProvider(provider)

	return noopShutdownFn, nil
}
