package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const portalLinkByMaskKeyPrefix = "portal_links_by_mask"

type CachedPortalLinkRepository struct {
	inner  datastore.PortalLinkRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedPortalLinkRepository(inner datastore.PortalLinkRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedPortalLinkRepository {
	return &CachedPortalLinkRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func portalLinkByMaskCacheKey(maskID string) string {
	return fmt.Sprintf("%s:%s", portalLinkByMaskKeyPrefix, maskID)
}

func (c *CachedPortalLinkRepository) FindPortalLinkByMaskId(ctx context.Context, maskID string) (*datastore.PortalLink, error) {
	key := portalLinkByMaskCacheKey(maskID)

	var pLink datastore.PortalLink
	err := c.cache.Get(ctx, key, &pLink)
	if err != nil {
		c.logger.Error("cache get error for portal link", "key", key, "error", err)
	}

	if pLink.UID != "" {
		return &pLink, nil
	}

	pl, err := c.inner.FindPortalLinkByMaskId(ctx, maskID)
	if err != nil {
		return nil, err
	}

	if setErr := c.cache.Set(ctx, key, pl, c.ttl); setErr != nil {
		c.logger.Error("cache set error for portal link", "key", key, "error", setErr)
	}

	return pl, nil
}

func (c *CachedPortalLinkRepository) UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *datastore.UpdatePortalLinkRequest) (*datastore.PortalLink, error) {
	result, err := c.inner.UpdatePortalLink(ctx, projectID, portalLink, request)
	if err != nil {
		return nil, err
	}

	c.invalidateByMask(ctx, portalLink.TokenMaskId)
	return result, nil
}

func (c *CachedPortalLinkRepository) RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error {
	// We don't have the maskID here, so TTL handles expiry.
	return c.inner.RevokePortalLink(ctx, projectID, portalLinkID)
}

func (c *CachedPortalLinkRepository) RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	// We don't have the maskID here, so TTL handles expiry.
	return c.inner.RefreshPortalLinkAuthToken(ctx, projectID, portalLinkID)
}

func (c *CachedPortalLinkRepository) invalidateByMask(ctx context.Context, maskID string) {
	if maskID == "" {
		return
	}
	key := portalLinkByMaskCacheKey(maskID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for portal link", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedPortalLinkRepository) CreatePortalLink(ctx context.Context, projectID string, request *datastore.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	return c.inner.CreatePortalLink(ctx, projectID, request)
}

func (c *CachedPortalLinkRepository) GetPortalLink(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	return c.inner.GetPortalLink(ctx, projectID, portalLinkID)
}

func (c *CachedPortalLinkRepository) GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	return c.inner.GetPortalLinkByToken(ctx, token)
}

func (c *CachedPortalLinkRepository) GetPortalLinkByOwnerID(ctx context.Context, projectID, ownerID string) (*datastore.PortalLink, error) {
	return c.inner.GetPortalLinkByOwnerID(ctx, projectID, ownerID)
}

func (c *CachedPortalLinkRepository) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	return c.inner.LoadPortalLinksPaged(ctx, projectID, filter, pageable)
}

func (c *CachedPortalLinkRepository) FindPortalLinksByOwnerID(ctx context.Context, ownerID string) ([]datastore.PortalLink, error) {
	return c.inner.FindPortalLinksByOwnerID(ctx, ownerID)
}
