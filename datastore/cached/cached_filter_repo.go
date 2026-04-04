package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const filterKeyPrefix = "filters"

type CachedFilterRepository struct {
	inner  datastore.FilterRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedFilterRepository(inner datastore.FilterRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedFilterRepository {
	return &CachedFilterRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func filterCacheKey(subscriptionID, eventType string) string {
	return fmt.Sprintf("%s:%s:%s", filterKeyPrefix, subscriptionID, eventType)
}

// cachedFilter wraps a pointer to distinguish "not-found cached" from "cache miss".
type cachedFilter struct {
	Filter *datastore.EventTypeFilter
	Found  bool
}

func (c *CachedFilterRepository) FindFilterBySubscriptionAndEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, error) {
	key := filterCacheKey(subscriptionID, eventType)

	var cached cachedFilter
	err := c.cache.Get(ctx, key, &cached)
	if err != nil {
		c.logger.Error("cache get error for filter", "key", key, "error", err)
	}

	if cached.Found {
		return cached.Filter, nil
	}

	// Check if we cached a not-found result
	// cached.Found == false && cached.Filter == nil could be either a miss or a cached not-found.
	// We use the Found flag to distinguish: if Found is false and we got no error, it's a miss.

	// Cache miss -- fetch from DB
	filter, err := c.inner.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, eventType)
	if err != nil {
		// Cache not-found results to avoid repeated DB lookups
		if err.Error() == datastore.ErrFilterNotFound.Error() {
			toCache := cachedFilter{Filter: nil, Found: true}
			if setErr := c.cache.Set(ctx, key, &toCache, c.ttl); setErr != nil {
				c.logger.Error("cache set error for filter not-found", "key", key, "error", setErr)
			}
		}
		return nil, err
	}

	toCache := cachedFilter{Filter: filter, Found: true}
	if setErr := c.cache.Set(ctx, key, &toCache, c.ttl); setErr != nil {
		c.logger.Error("cache set error for filter", "key", key, "error", setErr)
	}

	return filter, nil
}

func (c *CachedFilterRepository) FindFiltersBySubscriptionID(ctx context.Context, subscriptionID string) ([]datastore.EventTypeFilter, error) {
	return c.inner.FindFiltersBySubscriptionID(ctx, subscriptionID)
}

func (c *CachedFilterRepository) FindFilterByID(ctx context.Context, filterID string) (*datastore.EventTypeFilter, error) {
	return c.inner.FindFilterByID(ctx, filterID)
}

// Mutation methods -- invalidate related cache entries

func (c *CachedFilterRepository) CreateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	err := c.inner.CreateFilter(ctx, filter)
	if err != nil {
		return err
	}

	c.invalidateFilter(ctx, filter.SubscriptionID, filter.EventType)
	return nil
}

func (c *CachedFilterRepository) CreateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	err := c.inner.CreateFilters(ctx, filters)
	if err != nil {
		return err
	}

	for i := range filters {
		c.invalidateFilter(ctx, filters[i].SubscriptionID, filters[i].EventType)
	}
	return nil
}

func (c *CachedFilterRepository) UpdateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	err := c.inner.UpdateFilter(ctx, filter)
	if err != nil {
		return err
	}

	c.invalidateFilter(ctx, filter.SubscriptionID, filter.EventType)
	return nil
}

func (c *CachedFilterRepository) UpdateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	err := c.inner.UpdateFilters(ctx, filters)
	if err != nil {
		return err
	}

	for i := range filters {
		c.invalidateFilter(ctx, filters[i].SubscriptionID, filters[i].EventType)
	}
	return nil
}

func (c *CachedFilterRepository) DeleteFilter(ctx context.Context, filterID string) error {
	// We don't have subscriptionID/eventType here, so we can't do targeted invalidation.
	// The filter will expire naturally via TTL.
	return c.inner.DeleteFilter(ctx, filterID)
}

func (c *CachedFilterRepository) TestFilter(ctx context.Context, subscriptionID, eventType string, payload interface{}) (bool, error) {
	return c.inner.TestFilter(ctx, subscriptionID, eventType, payload)
}

func (c *CachedFilterRepository) invalidateFilter(ctx context.Context, subscriptionID, eventType string) {
	key := filterCacheKey(subscriptionID, eventType)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for filter", "key", key, "error", err)
	}

	// Also invalidate the catch-all since it may have been cached as not-found
	catchAllKey := filterCacheKey(subscriptionID, "*")
	if err := c.cache.Delete(ctx, catchAllKey); err != nil {
		c.logger.Error("cache delete error for catch-all filter", "key", catchAllKey, "error", err)
	}
}
