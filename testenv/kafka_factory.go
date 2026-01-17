package testenv

import (
	"context"
	"testing"
)

type KafkaConnectionFunc func(t *testing.T) string

func NewTestKafka(ctx context.Context) (*KafkaContainer, KafkaConnectionFunc, error) {
	container, err := StartKafka(ctx)
	if err != nil {
		return nil, nil, err
	}

	broker, err := container.GetBroker(ctx)
	if err != nil {
		return nil, nil, err
	}

	factory := func(t *testing.T) string {
		t.Helper()
		return broker
	}

	return container, factory, nil
}
