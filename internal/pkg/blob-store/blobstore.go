package blobstore

import (
	"context"
	"errors"
	"io"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// BlobStore defines the interface for uploading export data to a storage backend.
type BlobStore interface {
	Upload(ctx context.Context, key string, r io.Reader) error
}

// BlobStoreOptions holds configuration for connecting to a blob storage backend.
type BlobStoreOptions struct {
	Prefix       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	Region       string
	Endpoint     string
	SessionToken string

	OnPremStorageDir string

	AzureAccountName   string
	AzureAccountKey    string
	AzureContainerName string
	AzureEndpoint      string
}

// NewBlobStoreClient creates a BlobStore from the given storage policy configuration.
func NewBlobStoreClient(storage *datastore.StoragePolicyConfiguration, logger log.Logger) (BlobStore, error) {
	if storage == nil {
		return nil, errors.New("storage policy configuration is nil")
	}

	switch storage.Type {
	case datastore.S3:
		opts := BlobStoreOptions{
			Prefix:       storage.S3.Prefix.ValueOrZero(),
			Bucket:       storage.S3.Bucket.ValueOrZero(),
			Endpoint:     storage.S3.Endpoint.ValueOrZero(),
			AccessKey:    storage.S3.AccessKey.ValueOrZero(),
			SecretKey:    storage.S3.SecretKey.ValueOrZero(),
			SessionToken: storage.S3.SessionToken.ValueOrZero(),
			Region:       storage.S3.Region.ValueOrZero(),
		}
		return NewS3Client(opts, logger)

	case datastore.OnPrem:
		opts := BlobStoreOptions{
			OnPremStorageDir: storage.OnPrem.Path.String,
		}
		return NewOnPremClient(opts, logger)

	case datastore.AzureBlob:
		opts := BlobStoreOptions{
			Prefix:             storage.AzureBlob.Prefix.ValueOrZero(),
			AzureAccountName:   storage.AzureBlob.AccountName.ValueOrZero(),
			AzureAccountKey:    storage.AzureBlob.AccountKey.ValueOrZero(),
			AzureContainerName: storage.AzureBlob.ContainerName.ValueOrZero(),
			AzureEndpoint:      storage.AzureBlob.Endpoint.ValueOrZero(),
		}
		return NewAzureBlobClient(opts, logger)

	default:
		return nil, errors.New("invalid storage policy")
	}
}
