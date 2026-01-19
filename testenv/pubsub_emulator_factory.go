package testenv

import (
	"context"
	"testing"
)

type PubSubEmulatorHostFunc func(t *testing.T) string

func NewTestPubSubEmulator(ctx context.Context) (*PubSubEmulatorContainer, PubSubEmulatorHostFunc, error) {
	container, err := StartPubSubEmulator(ctx)
	if err != nil {
		return nil, nil, err
	}

	emulatorHost, err := container.GetEmulatorHost(ctx)
	if err != nil {
		return nil, nil, err
	}

	factory := func(t *testing.T) string {
		if t != nil {
			t.Helper()
		}
		return emulatorHost
	}

	return container, factory, nil
}
