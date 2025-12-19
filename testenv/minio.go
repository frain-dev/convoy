package testenv

import (
	"context"
	"fmt"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"

	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
	minioUsername  = "minioadmin"
	minioPassword  = "minioadmin"
	defaultBucket  = "convoy-test-exports"
	minioDockerTag = "RELEASE.2024-01-16T16-07-38Z"
)

// MinIOClientFunc is a factory function that creates a MinIO client for tests.
// It returns the client and the endpoint URL (with port).
type MinIOClientFunc func(t *testing.T) (*minio.Client, string, error)

// NewTestMinIO creates a new MinIO container and returns a factory function
// for creating MinIO clients in tests.
func NewTestMinIO(ctx context.Context) (*tcminio.MinioContainer, MinIOClientFunc, error) {
	container, err := tcminio.Run(ctx,
		"minio/minio:"+minioDockerTag,
		tcminio.WithUsername(minioUsername),
		tcminio.WithPassword(minioPassword),
		testcontainers.WithLogger(NewTestcontainersLogger()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start minio container: %w", err)
	}

	return container, newMinIOClientFunc(container), nil
}

func newMinIOClientFunc(container *tcminio.MinioContainer) MinIOClientFunc {
	return func(t *testing.T) (*minio.Client, string, error) {
		t.Helper()

		endpoint, err := container.ConnectionString(t.Context())
		if err != nil {
			return nil, "", fmt.Errorf("failed to get minio endpoint: %w", err)
		}

		// Create MinIO client with static credentials
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(minioUsername, minioPassword, ""),
			Secure: false, // Testcontainer uses HTTP
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to create minio client: %w", err)
		}

		// Create default bucket for tests
		ctx := t.Context()
		exists, err := client.BucketExists(ctx, defaultBucket)
		if err != nil {
			return nil, "", fmt.Errorf("failed to check if bucket exists: %w", err)
		}

		if !exists {
			err = client.MakeBucket(ctx, defaultBucket, minio.MakeBucketOptions{})
			if err != nil {
				return nil, "", fmt.Errorf("failed to create default bucket: %w", err)
			}
		}

		return client, endpoint, nil
	}
}
