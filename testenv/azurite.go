package testenv

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Azurite well-known development credentials
	azuriteAccountName = "devstoreaccount1"
	azuriteAccountKey  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	azuriteContainer   = "convoy-test-exports"
	azuriteBlobPort    = "10000/tcp"
	azuriteDockerImage = "mcr.microsoft.com/azure-storage/azurite:3.31.0"
)

// AzuriteClientFunc is a factory function that creates an Azure Blob client for tests.
// It returns the client and the blob service endpoint URL.
type AzuriteClientFunc func(t *testing.T) (*azblob.Client, string, error)

// NewTestAzurite creates a new Azurite container and returns a factory function
// for creating Azure Blob clients in tests.
func NewTestAzurite(ctx context.Context) (testcontainers.Container, AzuriteClientFunc, error) {
	req := testcontainers.ContainerRequest{
		Image:        azuriteDockerImage,
		ExposedPorts: []string{azuriteBlobPort},
		Cmd:          []string{"azurite-blob", "--blobHost", "0.0.0.0", "--blobPort", "10000", "--skipApiVersionCheck"},
		WaitingFor:   wait.ForListeningPort(azuriteBlobPort),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start azurite container: %w", err)
	}

	return container, newAzuriteClientFunc(container), nil
}

func newAzuriteClientFunc(container testcontainers.Container) AzuriteClientFunc {
	return func(t *testing.T) (*azblob.Client, string, error) {
		t.Helper()

		host, err := container.Host(t.Context())
		if err != nil {
			return nil, "", fmt.Errorf("failed to get azurite host: %w", err)
		}

		mappedPort, err := container.MappedPort(t.Context(), azuriteBlobPort)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get azurite port: %w", err)
		}

		endpoint := fmt.Sprintf("http://%s:%s/%s", host, mappedPort.Port(), azuriteAccountName)

		cred, err := azblob.NewSharedKeyCredential(azuriteAccountName, azuriteAccountKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create azurite credentials: %w", err)
		}

		client, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create azurite client: %w", err)
		}

		// Create default container for tests
		ctx := t.Context()
		_, err = client.CreateContainer(ctx, azuriteContainer, nil)
		if err != nil {
			// Ignore "container already exists" errors
			// Azurite returns a StorageError for this case
		}

		return client, endpoint, nil
	}
}
