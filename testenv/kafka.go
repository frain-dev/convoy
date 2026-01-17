package testenv

import (
	"context"

	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

type KafkaContainer struct {
	*kafka.KafkaContainer
}

func StartKafka(ctx context.Context) (*KafkaContainer, error) {
	kafkaContainer, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		return nil, err
	}

	return &KafkaContainer{KafkaContainer: kafkaContainer}, nil
}

func (k *KafkaContainer) GetBroker(ctx context.Context) (string, error) {
	brokers, err := k.Brokers(ctx)
	if err != nil {
		return "", err
	}
	if len(brokers) == 0 {
		return "", nil
	}
	return brokers[0], nil
}
