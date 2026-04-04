package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const apiKeyByMaskKeyPrefix = "apikeys_by_mask"

type CachedAPIKeyRepository struct {
	inner  datastore.APIKeyRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedAPIKeyRepository(inner datastore.APIKeyRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedAPIKeyRepository {
	return &CachedAPIKeyRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func apiKeyByMaskCacheKey(maskID string) string {
	return fmt.Sprintf("%s:%s", apiKeyByMaskKeyPrefix, maskID)
}

func (c *CachedAPIKeyRepository) GetAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	key := apiKeyByMaskCacheKey(maskID)

	var apiKey datastore.APIKey
	err := c.cache.Get(ctx, key, &apiKey)
	if err != nil {
		c.logger.Error("cache get error for api key", "key", key, "error", err)
	}

	if apiKey.UID != "" {
		return &apiKey, nil
	}

	ak, err := c.inner.GetAPIKeyByMaskID(ctx, maskID)
	if err != nil {
		return nil, err
	}

	if setErr := c.cache.Set(ctx, key, ak, c.ttl); setErr != nil {
		c.logger.Error("cache set error for api key", "key", key, "error", setErr)
	}

	return ak, nil
}

func (c *CachedAPIKeyRepository) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	err := c.inner.UpdateAPIKey(ctx, apiKey)
	if err != nil {
		return err
	}

	c.invalidateByMask(ctx, apiKey.MaskID)
	return nil
}

func (c *CachedAPIKeyRepository) RevokeAPIKeys(ctx context.Context, ids []string) error {
	// We don't have mask IDs here, so we can't do targeted invalidation.
	// Revoked keys will expire via TTL.
	return c.inner.RevokeAPIKeys(ctx, ids)
}

func (c *CachedAPIKeyRepository) invalidateByMask(ctx context.Context, maskID string) {
	if maskID == "" {
		return
	}
	key := apiKeyByMaskCacheKey(maskID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for api key", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedAPIKeyRepository) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return c.inner.CreateAPIKey(ctx, apiKey)
}

func (c *CachedAPIKeyRepository) GetAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error) {
	return c.inner.GetAPIKeyByID(ctx, id)
}

func (c *CachedAPIKeyRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	return c.inner.GetAPIKeyByHash(ctx, hash)
}

func (c *CachedAPIKeyRepository) GetAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	return c.inner.GetAPIKeyByProjectID(ctx, projectID)
}

func (c *CachedAPIKeyRepository) LoadAPIKeysPaged(ctx context.Context, filter *datastore.Filter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	return c.inner.LoadAPIKeysPaged(ctx, filter, pageable)
}
