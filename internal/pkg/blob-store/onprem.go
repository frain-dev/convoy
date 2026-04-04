package blobstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/frain-dev/convoy/pkg/logger"
)

// OnPremClient implements BlobStore for local filesystem storage.
type OnPremClient struct {
	opts   BlobStoreOptions
	logger log.Logger
}

// NewOnPremClient creates a new on-prem filesystem BlobStore.
func NewOnPremClient(opts BlobStoreOptions, logger log.Logger) (BlobStore, error) {
	return &OnPremClient{
		opts:   opts,
		logger: logger,
	}, nil
}

// Upload writes the stream to the local filesystem at the given key path.
func (o *OnPremClient) Upload(ctx context.Context, key string, r io.Reader) error {
	baseDir := filepath.Clean(o.opts.OnPremStorageDir)
	fullPath := filepath.Join(baseDir, filepath.Clean(key))

	// Guard against path traversal (e.g. key = "../../etc/passwd")
	if !strings.HasPrefix(fullPath, baseDir+string(filepath.Separator)) && fullPath != baseDir {
		return fmt.Errorf("path traversal detected: %q resolves outside base directory", key)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Errorf("create directory for %q: %w", fullPath, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file %q: %w", fullPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write to %q: %w", fullPath, err)
	}

	o.logger.Info(fmt.Sprintf("saved %q", fullPath))
	return nil
}
