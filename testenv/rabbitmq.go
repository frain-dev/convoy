package testenv

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type RabbitMQContainer struct {
	container testcontainers.Container
}

type RabbitMQConnectionFunc func(t *testing.T) (string, int, error)

// NewTestRabbitMQ creates a new RabbitMQ container for testing.
// Returns the container and a function to get connection details.
func NewTestRabbitMQ(ctx context.Context) (*RabbitMQContainer, RabbitMQConnectionFunc, error) {
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-management-alpine",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
		WaitingFor: wait.ForLog("Server startup complete").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           NewTestcontainersLogger(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start rabbitmq container: %w", err)
	}

	rmqContainer := &RabbitMQContainer{
		container: container,
	}

	return rmqContainer, newRabbitMQConnectionFunc(container), nil
}

// Terminate stops and removes the RabbitMQ container
func (r *RabbitMQContainer) Terminate(ctx context.Context) error {
	if r.container != nil {
		return r.container.Terminate(ctx)
	}
	return nil
}

// newRabbitMQConnectionFunc creates a factory function for getting RabbitMQ connection details
func newRabbitMQConnectionFunc(container testcontainers.Container) RabbitMQConnectionFunc {
	return func(t *testing.T) (string, int, error) {
		t.Helper()

		host, err := container.Host(t.Context())
		if err != nil {
			return "", 0, fmt.Errorf("failed to get rabbitmq host: %w", err)
		}

		mappedPort, err := container.MappedPort(t.Context(), "5672")
		if err != nil {
			return "", 0, fmt.Errorf("failed to get rabbitmq port: %w", err)
		}

		return host, mappedPort.Int(), nil
	}
}
