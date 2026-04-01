package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/flatten"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const subsByEndpointKeyPrefix = "subs_by_endpoint"

// cachedSubscriptions wraps a slice to distinguish "empty cached" from "cache miss".
type cachedSubscriptions struct {
	Subscriptions []datastore.Subscription
}

type CachedSubscriptionRepository struct {
	inner  datastore.SubscriptionRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedSubscriptionRepository(inner datastore.SubscriptionRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedSubscriptionRepository {
	return &CachedSubscriptionRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func subsByEndpointCacheKey(projectID, endpointID string) string {
	return fmt.Sprintf("%s:%s:%s", subsByEndpointKeyPrefix, projectID, endpointID)
}

func (c *CachedSubscriptionRepository) FindSubscriptionsByEndpointID(ctx context.Context, projectID string, endpointID string) ([]datastore.Subscription, error) {
	key := subsByEndpointCacheKey(projectID, endpointID)

	var cached cachedSubscriptions
	err := c.cache.Get(ctx, key, &cached)
	if err != nil {
		c.logger.Error("cache get error for subscriptions", "key", key, "error", err)
	}

	if cached.Subscriptions != nil {
		return cached.Subscriptions, nil
	}

	// Cache miss -- fetch from DB
	subs, err := c.inner.FindSubscriptionsByEndpointID(ctx, projectID, endpointID)
	if err != nil {
		return nil, err
	}

	toCache := cachedSubscriptions{Subscriptions: subs}
	if setErr := c.cache.Set(ctx, key, &toCache, c.ttl); setErr != nil {
		c.logger.Error("cache set error for subscriptions", "key", key, "error", setErr)
	}

	return subs, nil
}

func (c *CachedSubscriptionRepository) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	err := c.inner.CreateSubscription(ctx, projectID, subscription)
	if err != nil {
		return err
	}

	c.invalidateSubsByEndpoint(ctx, projectID, subscription.EndpointID)
	return nil
}

func (c *CachedSubscriptionRepository) UpdateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	err := c.inner.UpdateSubscription(ctx, projectID, subscription)
	if err != nil {
		return err
	}

	c.invalidateSubsByEndpoint(ctx, projectID, subscription.EndpointID)
	return nil
}

func (c *CachedSubscriptionRepository) DeleteSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	err := c.inner.DeleteSubscription(ctx, projectID, subscription)
	if err != nil {
		return err
	}

	c.invalidateSubsByEndpoint(ctx, projectID, subscription.EndpointID)
	return nil
}

func (c *CachedSubscriptionRepository) invalidateSubsByEndpoint(ctx context.Context, projectID, endpointID string) {
	if endpointID == "" {
		return
	}
	key := subsByEndpointCacheKey(projectID, endpointID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for subscriptions", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedSubscriptionRepository) LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	return c.inner.LoadSubscriptionsPaged(ctx, projectID, filter, pageable)
}

func (c *CachedSubscriptionRepository) FindSubscriptionByID(ctx context.Context, projectID, id string) (*datastore.Subscription, error) {
	return c.inner.FindSubscriptionByID(ctx, projectID, id)
}

func (c *CachedSubscriptionRepository) FindSubscriptionsBySourceID(ctx context.Context, projectID, sourceID string) ([]datastore.Subscription, error) {
	return c.inner.FindSubscriptionsBySourceID(ctx, projectID, sourceID)
}

func (c *CachedSubscriptionRepository) FindCLISubscriptions(ctx context.Context, projectID string) ([]datastore.Subscription, error) {
	return c.inner.FindCLISubscriptions(ctx, projectID)
}

func (c *CachedSubscriptionRepository) CountEndpointSubscriptions(ctx context.Context, a string, b string, d string) (int64, error) {
	return c.inner.CountEndpointSubscriptions(ctx, a, b, d)
}

func (c *CachedSubscriptionRepository) TestSubscriptionFilter(ctx context.Context, payload, filter interface{}, isFlattened bool) (bool, error) {
	return c.inner.TestSubscriptionFilter(ctx, payload, filter, isFlattened)
}

func (c *CachedSubscriptionRepository) CompareFlattenedPayload(ctx context.Context, payload, filter flatten.M, isFlattened bool) (bool, error) {
	return c.inner.CompareFlattenedPayload(ctx, payload, filter, isFlattened)
}

func (c *CachedSubscriptionRepository) LoadAllSubscriptionConfig(ctx context.Context, projectIDs []string, pageSize int64) ([]datastore.Subscription, error) {
	return c.inner.LoadAllSubscriptionConfig(ctx, projectIDs, pageSize)
}

func (c *CachedSubscriptionRepository) FetchDeletedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	return c.inner.FetchDeletedSubscriptions(ctx, projectIDs, subscriptionUpdates, pageSize)
}

func (c *CachedSubscriptionRepository) FetchUpdatedSubscriptions(ctx context.Context, projectIDs []string, subscriptionUpdates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	return c.inner.FetchUpdatedSubscriptions(ctx, projectIDs, subscriptionUpdates, pageSize)
}

func (c *CachedSubscriptionRepository) FetchNewSubscriptions(ctx context.Context, projectIDs []string, knownSubscriptionIDs []string, lastSyncTime time.Time, pageSize int64) ([]datastore.Subscription, error) {
	return c.inner.FetchNewSubscriptions(ctx, projectIDs, knownSubscriptionIDs, lastSyncTime, pageSize)
}
