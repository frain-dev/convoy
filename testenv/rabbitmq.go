package testenv

import (
	"context"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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

// Restart stops and restarts the RabbitMQ container.
// This is useful for testing reconnection logic.
func (r *RabbitMQContainer) Restart(ctx context.Context) error {
	if r.container == nil {
		return fmt.Errorf("container is nil")
	}

	// Stop the container
	if err := r.container.Stop(ctx, nil); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Start the container again
	if err := r.container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for RabbitMQ to be fully ready to accept connections
	// The testcontainers wait strategy may see old logs, so we need to explicitly wait
	host, err := r.container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get host: %w", err)
	}

	mappedPort, err := r.container.MappedPort(ctx, "5672")
	if err != nil {
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Try to connect to RabbitMQ and verify it's operational for up to 90 seconds
	connStr := fmt.Sprintf("amqp://guest:guest@%s:%d/", host, mappedPort.Int())
	deadline := time.Now().Add(90 * time.Second)

	for time.Now().Before(deadline) {
		if err := verifyRabbitMQReady(connStr); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("RabbitMQ did not become ready after restart within 90 seconds")
}

// verifyRabbitMQReady checks if RabbitMQ is ready by connecting and opening a channel
func verifyRabbitMQReady(connStr string) error {
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Try to open a channel to verify RabbitMQ is fully operational
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	ch.Close()

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
