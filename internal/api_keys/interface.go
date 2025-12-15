package api_keys

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys/models"
)

// Service defines the interface for API key operations
type Service interface {
	// CreateAPIKey creates a new API key
	CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error

	// UpdateAPIKey updates an existing API key
	UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error

	// GetAPIKeyByID retrieves an API key by its ID
	GetAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error)

	// GetAPIKeyByMaskID retrieves an API key by its mask ID (used for authentication)
	GetAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error)

	// GetAPIKeyByHash retrieves an API key by its hash
	GetAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error)

	// GetAPIKeyByProjectID retrieves an API key by its project ID
	GetAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error)

	// RevokeAPIKeys revokes (soft deletes) multiple API keys
	RevokeAPIKeys(ctx context.Context, ids []string) error

	// LoadAPIKeysPaged retrieves API keys with pagination and filtering
	LoadAPIKeysPaged(ctx context.Context, filter *models.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error)
}
