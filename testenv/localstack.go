package testenv

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type LocalStackContainer struct {
	testcontainers.Container
}

// StartLocalStack starts a LocalStack container for AWS service emulation
func StartLocalStack(ctx context.Context) (*LocalStackContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "localstack/localstack:latest",
		ExposedPorts: []string{"4566/tcp"},
		Env: map[string]string{
			"SERVICES":              "sqs",
			"DEFAULT_REGION":        "us-east-1",
			"EAGER_SERVICE_LOADING": "1",
			"DEBUG":                 "0",
		},
		WaitingFor: wait.ForLog("Ready."),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, err
	}

	return &LocalStackContainer{Container: container}, nil
}

// GetEndpoint returns the LocalStack endpoint URL for AWS SDK configuration
func (l *LocalStackContainer) GetEndpoint(ctx context.Context) (string, error) {
	host, err := l.Host(ctx)
	if err != nil {
		return "", err
	}

	port, err := l.MappedPort(ctx, "4566")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}
