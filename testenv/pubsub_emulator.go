package testenv

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PubSubEmulatorContainer struct {
	testcontainers.Container
}

func StartPubSubEmulator(ctx context.Context) (*PubSubEmulatorContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators",
		ExposedPorts: []string{"8085/tcp"},
		Cmd: []string{
			"gcloud",
			"beta",
			"emulators",
			"pubsub",
			"start",
			"--host-port=0.0.0.0:8085",
		},
		WaitingFor: wait.ForLog("Server started"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, err
	}

	return &PubSubEmulatorContainer{Container: container}, nil
}

func (p *PubSubEmulatorContainer) GetEmulatorHost(ctx context.Context) (string, error) {
	host, err := p.Host(ctx)
	if err != nil {
		return "", err
	}

	port, err := p.MappedPort(ctx, "8085")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", host, port.Port()), nil
}
