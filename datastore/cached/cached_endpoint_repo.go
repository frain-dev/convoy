package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const (
	endpointKeyPrefix         = "endpoints"
	endpointsByOwnerKeyPrefix = "endpoints_by_owner"
)

type CachedEndpointRepository struct {
	inner  datastore.EndpointRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedEndpointRepository(inner datastore.EndpointRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedEndpointRepository {
	return &CachedEndpointRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func endpointCacheKey(projectID, endpointID string) string {
	return fmt.Sprintf("%s:%s:%s", endpointKeyPrefix, projectID, endpointID)
}

func endpointsByOwnerCacheKey(projectID, ownerID string) string {
	return fmt.Sprintf("%s:%s:%s", endpointsByOwnerKeyPrefix, projectID, ownerID)
}

// cachedEndpoints wraps a slice to distinguish "empty cached" from "cache miss".
type cachedEndpoints struct {
	Endpoints []datastore.Endpoint
}

func (c *CachedEndpointRepository) FindEndpointByID(ctx context.Context, id string, projectID string) (*datastore.Endpoint, error) {
	key := endpointCacheKey(projectID, id)

	var endpoint datastore.Endpoint
	err := c.cache.Get(ctx, key, &endpoint)
	if err != nil {
		c.logger.Error("cache get error for endpoint", "key", key, "error", err)
	}

	if endpoint.UID != "" {
		return &endpoint, nil
	}

	// Cache miss -- fetch from DB
	ep, err := c.inner.FindEndpointByID(ctx, id, projectID)
	if err != nil {
		return nil, err
	}

	if setErr := c.cache.Set(ctx, key, ep, c.ttl); setErr != nil {
		c.logger.Error("cache set error for endpoint", "key", key, "error", setErr)
	}

	return ep, nil
}

func (c *CachedEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := c.inner.UpdateEndpoint(ctx, endpoint, projectID)
	if err != nil {
		return err
	}

	c.invalidateEndpoint(ctx, projectID, endpoint.UID)
	c.invalidateEndpointsByOwner(ctx, projectID, endpoint.OwnerID)
	return nil
}

func (c *CachedEndpointRepository) UpdateEndpointStatus(ctx context.Context, projectID, endpointID string, status datastore.EndpointStatus) error {
	err := c.inner.UpdateEndpointStatus(ctx, projectID, endpointID, status)
	if err != nil {
		return err
	}

	c.invalidateEndpoint(ctx, projectID, endpointID)
	return nil
}

func (c *CachedEndpointRepository) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := c.inner.DeleteEndpoint(ctx, endpoint, projectID)
	if err != nil {
		return err
	}

	c.invalidateEndpoint(ctx, projectID, endpoint.UID)
	c.invalidateEndpointsByOwner(ctx, projectID, endpoint.OwnerID)
	return nil
}

func (c *CachedEndpointRepository) UpdateSecrets(ctx context.Context, endpointID string, projectID string, secrets datastore.Secrets) error {
	err := c.inner.UpdateSecrets(ctx, endpointID, projectID, secrets)
	if err != nil {
		return err
	}

	c.invalidateEndpoint(ctx, projectID, endpointID)
	return nil
}

func (c *CachedEndpointRepository) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID string, projectID string) error {
	err := c.inner.DeleteSecret(ctx, endpoint, secretID, projectID)
	if err != nil {
		return err
	}

	c.invalidateEndpoint(ctx, projectID, endpoint.UID)
	return nil
}

func (c *CachedEndpointRepository) invalidateEndpoint(ctx context.Context, projectID, endpointID string) {
	key := endpointCacheKey(projectID, endpointID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for endpoint", "key", key, "error", err)
	}
}

func (c *CachedEndpointRepository) invalidateEndpointsByOwner(ctx context.Context, projectID, ownerID string) {
	if ownerID == "" {
		return
	}
	key := endpointsByOwnerCacheKey(projectID, ownerID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for endpoints by owner", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedEndpointRepository) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := c.inner.CreateEndpoint(ctx, endpoint, projectID)
	if err != nil {
		return err
	}

	c.invalidateEndpointsByOwner(ctx, projectID, endpoint.OwnerID)
	return nil
}

func (c *CachedEndpointRepository) FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]datastore.Endpoint, error) {
	return c.inner.FindEndpointsByID(ctx, ids, projectID)
}

func (c *CachedEndpointRepository) FindEndpointsByAppID(ctx context.Context, appID string, projectID string) ([]datastore.Endpoint, error) {
	return c.inner.FindEndpointsByAppID(ctx, appID, projectID)
}

func (c *CachedEndpointRepository) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	key := endpointsByOwnerCacheKey(projectID, ownerID)

	var cached cachedEndpoints
	err := c.cache.Get(ctx, key, &cached)
	if err != nil {
		c.logger.Error("cache get error for endpoints by owner", "key", key, "error", err)
	}

	if cached.Endpoints != nil {
		return cached.Endpoints, nil
	}

	// Cache miss -- fetch from DB
	eps, err := c.inner.FindEndpointsByOwnerID(ctx, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	toCache := cachedEndpoints{Endpoints: eps}
	if setErr := c.cache.Set(ctx, key, &toCache, c.ttl); setErr != nil {
		c.logger.Error("cache set error for endpoints by owner", "key", key, "error", setErr)
	}

	return eps, nil
}

func (c *CachedEndpointRepository) FetchEndpointIDsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]string, error) {
	return c.inner.FetchEndpointIDsByOwnerID(ctx, projectID, ownerID)
}

func (c *CachedEndpointRepository) FindEndpointByTargetURL(ctx context.Context, projectID string, targetURL string) (*datastore.Endpoint, error) {
	return c.inner.FindEndpointByTargetURL(ctx, projectID, targetURL)
}

func (c *CachedEndpointRepository) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	return c.inner.CountProjectEndpoints(ctx, projectID)
}

func (c *CachedEndpointRepository) LoadEndpointsPaged(ctx context.Context, projectID string, filter *datastore.Filter, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	return c.inner.LoadEndpointsPaged(ctx, projectID, filter, pageable)
}
