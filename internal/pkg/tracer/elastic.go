package tracer

import (
	"go.elastic.co/apm/module/apmotel/v2"
	"go.opentelemetry.io/otel"
)

type ElasticTracer struct{}

func (et *ElasticTracer) Init(componentName string) error {
	provider, err := apmotel.NewTracerProvider()
	if err != nil {
		return err
	}

	// Configure Tracer Provider.
	otel.SetTracerProvider(provider)

	return nil
}
