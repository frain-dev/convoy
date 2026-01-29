package testenv

import (
	"context"
	"testing"
)

// LocalStackConnectionFunc is a factory function that returns a LocalStack endpoint for tests
type LocalStackConnectionFunc func(t *testing.T) string

// NewTestLocalStack starts a LocalStack container and returns a connection factory
func NewTestLocalStack(ctx context.Context) (*LocalStackContainer, LocalStackConnectionFunc, error) {
	container, err := StartLocalStack(ctx)
	if err != nil {
		return nil, nil, err
	}

	endpoint, err := container.GetEndpoint(ctx)
	if err != nil {
		return nil, nil, err
	}

	factory := func(t *testing.T) string {
		t.Helper()
		return endpoint
	}

	return container, factory, nil
}
