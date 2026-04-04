package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const orgKeyPrefix = "organisations"

type CachedOrganisationRepository struct {
	inner  datastore.OrganisationRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedOrganisationRepository(inner datastore.OrganisationRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedOrganisationRepository {
	return &CachedOrganisationRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func orgCacheKey(orgID string) string {
	return fmt.Sprintf("%s:%s", orgKeyPrefix, orgID)
}

func (c *CachedOrganisationRepository) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	key := orgCacheKey(id)

	var org datastore.Organisation
	err := c.cache.Get(ctx, key, &org)
	if err != nil {
		c.logger.Error("cache get error for organisation", "key", key, "error", err)
	}

	if org.UID != "" {
		return &org, nil
	}

	o, err := c.inner.FetchOrganisationByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if setErr := c.cache.Set(ctx, key, o, c.ttl); setErr != nil {
		c.logger.Error("cache set error for organisation", "key", key, "error", setErr)
	}

	return o, nil
}

func (c *CachedOrganisationRepository) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	err := c.inner.UpdateOrganisation(ctx, org)
	if err != nil {
		return err
	}

	c.invalidateOrg(ctx, org.UID)
	return nil
}

func (c *CachedOrganisationRepository) UpdateOrganisationLicenseData(ctx context.Context, orgID, licenseData string) error {
	err := c.inner.UpdateOrganisationLicenseData(ctx, orgID, licenseData)
	if err != nil {
		return err
	}

	c.invalidateOrg(ctx, orgID)
	return nil
}

func (c *CachedOrganisationRepository) DeleteOrganisation(ctx context.Context, id string) error {
	err := c.inner.DeleteOrganisation(ctx, id)
	if err != nil {
		return err
	}

	c.invalidateOrg(ctx, id)
	return nil
}

func (c *CachedOrganisationRepository) invalidateOrg(ctx context.Context, orgID string) {
	key := orgCacheKey(orgID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for organisation", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedOrganisationRepository) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	return c.inner.CreateOrganisation(ctx, org)
}

func (c *CachedOrganisationRepository) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	return c.inner.FetchOrganisationByCustomDomain(ctx, domain)
}

func (c *CachedOrganisationRepository) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	return c.inner.FetchOrganisationByAssignedDomain(ctx, domain)
}

func (c *CachedOrganisationRepository) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	return c.inner.LoadOrganisationsPaged(ctx, pageable)
}

func (c *CachedOrganisationRepository) LoadOrganisationsPagedWithSearch(ctx context.Context, pageable datastore.Pageable, search string) ([]datastore.Organisation, datastore.PaginationData, error) {
	return c.inner.LoadOrganisationsPagedWithSearch(ctx, pageable, search)
}

func (c *CachedOrganisationRepository) CountOrganisations(ctx context.Context) (int64, error) {
	return c.inner.CountOrganisations(ctx)
}

func (c *CachedOrganisationRepository) CalculateUsage(ctx context.Context, orgID string, startTime, endTime time.Time) (*datastore.OrganisationUsage, error) {
	return c.inner.CalculateUsage(ctx, orgID, startTime, endTime)
}
