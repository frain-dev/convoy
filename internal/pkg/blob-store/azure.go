package blobstore

import (
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	log "github.com/frain-dev/convoy/pkg/logger"
)

// AzureBlobClient implements BlobStore for Azure Blob Storage.
type AzureBlobClient struct {
	client        *azblob.Client
	containerName string
	prefix        string
	logger        log.Logger
}

// NewAzureBlobClient creates a new Azure Blob Storage BlobStore.
func NewAzureBlobClient(opts BlobStoreOptions, logger log.Logger) (BlobStore, error) {
	serviceURL := opts.AzureEndpoint
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net", opts.AzureAccountName)
	}

	cred, err := azblob.NewSharedKeyCredential(opts.AzureAccountName, opts.AzureAccountKey)
	if err != nil {
		return nil, fmt.Errorf("azure credentials: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure client: %w", err)
	}

	return &AzureBlobClient{
		client:        client,
		containerName: opts.AzureContainerName,
		prefix:        opts.Prefix,
		logger:        logger,
	}, nil
}

// Upload streams data to Azure Blob Storage.
func (a *AzureBlobClient) Upload(ctx context.Context, key string, r io.Reader) error {
	blobName := key
	if a.prefix != "" {
		blobName = a.prefix + "/" + key
	}

	_, err := a.client.UploadStream(ctx, a.containerName, blobName, r,
		&azblob.UploadStreamOptions{
			BlockSize:   8 * 1024 * 1024, // 8MB per block
			Concurrency: 3,
		})
	if err != nil {
		return fmt.Errorf("azure upload %q: %w", blobName, err)
	}

	a.logger.Info(fmt.Sprintf("uploaded %q to azure container %q", blobName, a.containerName))
	return nil
}
