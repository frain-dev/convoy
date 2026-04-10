package cached

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/cachedrepo"
	"github.com/frain-dev/convoy/pkg/flatten"
)

// ============================================================================
// ProjectRepository
// ============================================================================

type CachedProjectRepository struct {
	inner  datastore.ProjectRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedProjectRepository(inner datastore.ProjectRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedProjectRepository {
	return &CachedProjectRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedProjectRepository) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	return cachedrepo.FetchOne(ctx, r.cache, r.logger, "projects:"+id, r.ttl,
		func(p *datastore.Project) bool { return p.UID != "" },
		func() (*datastore.Project, error) { return r.inner.FetchProjectByID(ctx, id) })
}

func (r *CachedProjectRepository) UpdateProject(ctx context.Context, project *datastore.Project) error {
	err := r.inner.UpdateProject(ctx, project)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "projects:"+project.UID)
	}
	return err
}

func (r *CachedProjectRepository) DeleteProject(ctx context.Context, uid string) error {
	err := r.inner.DeleteProject(ctx, uid)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "projects:"+uid)
	}
	return err
}

func (r *CachedProjectRepository) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	return r.inner.LoadProjects(ctx, f)
}
func (r *CachedProjectRepository) CreateProject(ctx context.Context, p *datastore.Project) error {
	return r.inner.CreateProject(ctx, p)
}
func (r *CachedProjectRepository) CountProjects(ctx context.Context) (int64, error) {
	return r.inner.CountProjects(ctx)
}
func (r *CachedProjectRepository) GetProjectsWithEventsInTheInterval(ctx context.Context, interval int) ([]datastore.ProjectEvents, error) {
	return r.inner.GetProjectsWithEventsInTheInterval(ctx, interval)
}
func (r *CachedProjectRepository) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	return r.inner.FillProjectsStatistics(ctx, project)
}

// ============================================================================
// EndpointRepository
// ============================================================================

type CachedEndpointRepository struct {
	inner  datastore.EndpointRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedEndpointRepository(inner datastore.EndpointRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedEndpointRepository {
	return &CachedEndpointRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedEndpointRepository) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	return cachedrepo.FetchOne(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+id, r.ttl,
		func(e *datastore.Endpoint) bool { return e.UID != "" },
		func() (*datastore.Endpoint, error) { return r.inner.FindEndpointByID(ctx, id, projectID) })
}

func (r *CachedEndpointRepository) FindEndpointsByOwnerID(ctx context.Context, projectID, ownerID string) ([]datastore.Endpoint, error) {
	return cachedrepo.FetchSlice(ctx, r.cache, r.logger, "endpoints_by_owner:"+projectID+":"+ownerID, r.ttl,
		func() ([]datastore.Endpoint, error) { return r.inner.FindEndpointsByOwnerID(ctx, projectID, ownerID) })
}

func (r *CachedEndpointRepository) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := r.inner.CreateEndpoint(ctx, endpoint, projectID)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints_by_owner:"+projectID+":"+endpoint.OwnerID)
	}
	return err
}

func (r *CachedEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := r.inner.UpdateEndpoint(ctx, endpoint, projectID)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+endpoint.UID, "endpoints_by_owner:"+projectID+":"+endpoint.OwnerID)
	}
	return err
}

func (r *CachedEndpointRepository) UpdateEndpointStatus(ctx context.Context, projectID, endpointID string, status datastore.EndpointStatus) error {
	err := r.inner.UpdateEndpointStatus(ctx, projectID, endpointID, status)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+endpointID)
	}
	return err
}

func (r *CachedEndpointRepository) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	err := r.inner.DeleteEndpoint(ctx, endpoint, projectID)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+endpoint.UID, "endpoints_by_owner:"+projectID+":"+endpoint.OwnerID)
	}
	return err
}

func (r *CachedEndpointRepository) UpdateSecrets(ctx context.Context, endpointID, projectID string, secrets datastore.Secrets) error {
	err := r.inner.UpdateSecrets(ctx, endpointID, projectID, secrets)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+endpointID)
	}
	return err
}

func (r *CachedEndpointRepository) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID, projectID string) error {
	err := r.inner.DeleteSecret(ctx, endpoint, secretID, projectID)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "endpoints:"+projectID+":"+endpoint.UID)
	}
	return err
}

func (r *CachedEndpointRepository) FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]datastore.Endpoint, error) {
	return r.inner.FindEndpointsByID(ctx, ids, projectID)
}
func (r *CachedEndpointRepository) FindEndpointsByAppID(ctx context.Context, appID, projectID string) ([]datastore.Endpoint, error) {
	return r.inner.FindEndpointsByAppID(ctx, appID, projectID)
}
func (r *CachedEndpointRepository) FetchEndpointIDsByOwnerID(ctx context.Context, projectID, ownerID string) ([]string, error) {
	return r.inner.FetchEndpointIDsByOwnerID(ctx, projectID, ownerID)
}
func (r *CachedEndpointRepository) FindEndpointByTargetURL(ctx context.Context, projectID, targetURL string) (*datastore.Endpoint, error) {
	return r.inner.FindEndpointByTargetURL(ctx, projectID, targetURL)
}
func (r *CachedEndpointRepository) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	return r.inner.CountProjectEndpoints(ctx, projectID)
}
func (r *CachedEndpointRepository) LoadEndpointsPaged(ctx context.Context, projectID string, filter *datastore.Filter, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	return r.inner.LoadEndpointsPaged(ctx, projectID, filter, pageable)
}

// ============================================================================
// SubscriptionRepository
// ============================================================================

type CachedSubscriptionRepository struct {
	inner  datastore.SubscriptionRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedSubscriptionRepository(inner datastore.SubscriptionRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedSubscriptionRepository {
	return &CachedSubscriptionRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedSubscriptionRepository) FindSubscriptionsByEndpointID(ctx context.Context, projectID, endpointID string) ([]datastore.Subscription, error) {
	return cachedrepo.FetchSlice(ctx, r.cache, r.logger, "subs_by_endpoint:"+projectID+":"+endpointID, r.ttl,
		func() ([]datastore.Subscription, error) {
			return r.inner.FindSubscriptionsByEndpointID(ctx, projectID, endpointID)
		})
}

func (r *CachedSubscriptionRepository) CreateSubscription(ctx context.Context, projectID string, sub *datastore.Subscription) error {
	err := r.inner.CreateSubscription(ctx, projectID, sub)
	if err == nil && sub.EndpointID != "" {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "subs_by_endpoint:"+projectID+":"+sub.EndpointID)
	}
	return err
}

func (r *CachedSubscriptionRepository) UpdateSubscription(ctx context.Context, projectID string, sub *datastore.Subscription) error {
	err := r.inner.UpdateSubscription(ctx, projectID, sub)
	if err == nil && sub.EndpointID != "" {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "subs_by_endpoint:"+projectID+":"+sub.EndpointID)
	}
	return err
}

func (r *CachedSubscriptionRepository) DeleteSubscription(ctx context.Context, projectID string, sub *datastore.Subscription) error {
	err := r.inner.DeleteSubscription(ctx, projectID, sub)
	if err == nil && sub.EndpointID != "" {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "subs_by_endpoint:"+projectID+":"+sub.EndpointID)
	}
	return err
}

func (r *CachedSubscriptionRepository) LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	return r.inner.LoadSubscriptionsPaged(ctx, projectID, filter, pageable)
}
func (r *CachedSubscriptionRepository) FindSubscriptionByID(ctx context.Context, projectID, id string) (*datastore.Subscription, error) {
	return r.inner.FindSubscriptionByID(ctx, projectID, id)
}
func (r *CachedSubscriptionRepository) FindSubscriptionsBySourceID(ctx context.Context, projectID, sourceID string) ([]datastore.Subscription, error) {
	return r.inner.FindSubscriptionsBySourceID(ctx, projectID, sourceID)
}
func (r *CachedSubscriptionRepository) FindCLISubscriptions(ctx context.Context, projectID string) ([]datastore.Subscription, error) {
	return r.inner.FindCLISubscriptions(ctx, projectID)
}
func (r *CachedSubscriptionRepository) CountEndpointSubscriptions(ctx context.Context, a, b, d string) (int64, error) {
	return r.inner.CountEndpointSubscriptions(ctx, a, b, d)
}
func (r *CachedSubscriptionRepository) TestSubscriptionFilter(ctx context.Context, payload, filter interface{}, isFlattened bool) (bool, error) {
	return r.inner.TestSubscriptionFilter(ctx, payload, filter, isFlattened)
}
func (r *CachedSubscriptionRepository) CompareFlattenedPayload(ctx context.Context, payload, filter flatten.M, isFlattened bool) (bool, error) {
	return r.inner.CompareFlattenedPayload(ctx, payload, filter, isFlattened)
}
func (r *CachedSubscriptionRepository) LoadAllSubscriptionConfig(ctx context.Context, projectIDs []string, pageSize int64) ([]datastore.Subscription, error) {
	return r.inner.LoadAllSubscriptionConfig(ctx, projectIDs, pageSize)
}
func (r *CachedSubscriptionRepository) FetchDeletedSubscriptions(ctx context.Context, projectIDs []string, updates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	return r.inner.FetchDeletedSubscriptions(ctx, projectIDs, updates, pageSize)
}
func (r *CachedSubscriptionRepository) FetchUpdatedSubscriptions(ctx context.Context, projectIDs []string, updates []datastore.SubscriptionUpdate, pageSize int64) ([]datastore.Subscription, error) {
	return r.inner.FetchUpdatedSubscriptions(ctx, projectIDs, updates, pageSize)
}
func (r *CachedSubscriptionRepository) FetchNewSubscriptions(ctx context.Context, projectIDs, knownIDs []string, lastSyncTime time.Time, pageSize int64) ([]datastore.Subscription, error) {
	return r.inner.FetchNewSubscriptions(ctx, projectIDs, knownIDs, lastSyncTime, pageSize)
}

// ============================================================================
// FilterRepository
// ============================================================================

type CachedFilterRepository struct {
	inner  datastore.FilterRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedFilterRepository(inner datastore.FilterRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedFilterRepository {
	return &CachedFilterRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedFilterRepository) FindFilterBySubscriptionAndEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, error) {
	return cachedrepo.FetchWithNotFound(ctx, r.cache, r.logger, "filters:"+subscriptionID+":"+eventType, r.ttl,
		func() (*datastore.EventTypeFilter, error) {
			return r.inner.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, eventType)
		},
		func(err error) bool { return err.Error() == datastore.ErrFilterNotFound.Error() })
}

func (r *CachedFilterRepository) CreateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	err := r.inner.CreateFilter(ctx, filter)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "filters:"+filter.SubscriptionID+":"+filter.EventType, "filters:"+filter.SubscriptionID+":*")
	}
	return err
}

func (r *CachedFilterRepository) CreateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	err := r.inner.CreateFilters(ctx, filters)
	if err == nil {
		for i := range filters {
			cachedrepo.Invalidate(ctx, r.cache, r.logger, "filters:"+filters[i].SubscriptionID+":"+filters[i].EventType, "filters:"+filters[i].SubscriptionID+":*")
		}
	}
	return err
}

func (r *CachedFilterRepository) UpdateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	err := r.inner.UpdateFilter(ctx, filter)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "filters:"+filter.SubscriptionID+":"+filter.EventType, "filters:"+filter.SubscriptionID+":*")
	}
	return err
}

func (r *CachedFilterRepository) UpdateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	err := r.inner.UpdateFilters(ctx, filters)
	if err == nil {
		for i := range filters {
			cachedrepo.Invalidate(ctx, r.cache, r.logger, "filters:"+filters[i].SubscriptionID+":"+filters[i].EventType, "filters:"+filters[i].SubscriptionID+":*")
		}
	}
	return err
}

func (r *CachedFilterRepository) DeleteFilter(ctx context.Context, filterID string) error {
	// DeleteFilter lacks subscriptionID/eventType for targeted invalidation — TTL handles it
	return r.inner.DeleteFilter(ctx, filterID)
}
func (r *CachedFilterRepository) FindFilterByID(ctx context.Context, filterID string) (*datastore.EventTypeFilter, error) {
	return r.inner.FindFilterByID(ctx, filterID)
}
func (r *CachedFilterRepository) FindFiltersBySubscriptionID(ctx context.Context, subscriptionID string) ([]datastore.EventTypeFilter, error) {
	return r.inner.FindFiltersBySubscriptionID(ctx, subscriptionID)
}
func (r *CachedFilterRepository) TestFilter(ctx context.Context, subscriptionID, eventType string, payload interface{}) (bool, error) {
	return r.inner.TestFilter(ctx, subscriptionID, eventType, payload)
}

// ============================================================================
// APIKeyRepository
// ============================================================================

type CachedAPIKeyRepository struct {
	inner  datastore.APIKeyRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedAPIKeyRepository(inner datastore.APIKeyRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedAPIKeyRepository {
	return &CachedAPIKeyRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedAPIKeyRepository) GetAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	return cachedrepo.FetchOne(ctx, r.cache, r.logger, "apikeys_by_mask:"+maskID, r.ttl,
		func(a *datastore.APIKey) bool { return a.UID != "" },
		func() (*datastore.APIKey, error) { return r.inner.GetAPIKeyByMaskID(ctx, maskID) })
}

func (r *CachedAPIKeyRepository) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	err := r.inner.UpdateAPIKey(ctx, apiKey)
	if err == nil && apiKey.MaskID != "" {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "apikeys_by_mask:"+apiKey.MaskID)
	}
	return err
}

func (r *CachedAPIKeyRepository) CreateAPIKey(ctx context.Context, a *datastore.APIKey) error {
	return r.inner.CreateAPIKey(ctx, a)
}
func (r *CachedAPIKeyRepository) RevokeAPIKeys(ctx context.Context, ids []string) error {
	return r.inner.RevokeAPIKeys(ctx, ids)
}
func (r *CachedAPIKeyRepository) GetAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error) {
	return r.inner.GetAPIKeyByID(ctx, id)
}
func (r *CachedAPIKeyRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	return r.inner.GetAPIKeyByHash(ctx, hash)
}
func (r *CachedAPIKeyRepository) GetAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	return r.inner.GetAPIKeyByProjectID(ctx, projectID)
}
func (r *CachedAPIKeyRepository) LoadAPIKeysPaged(ctx context.Context, filter *datastore.Filter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	return r.inner.LoadAPIKeysPaged(ctx, filter, pageable)
}

// ============================================================================
// PortalLinkRepository
// ============================================================================

type CachedPortalLinkRepository struct {
	inner  datastore.PortalLinkRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedPortalLinkRepository(inner datastore.PortalLinkRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedPortalLinkRepository {
	return &CachedPortalLinkRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedPortalLinkRepository) FindPortalLinkByMaskId(ctx context.Context, maskID string) (*datastore.PortalLink, error) {
	return cachedrepo.FetchOne(ctx, r.cache, r.logger, "portal_links_by_mask:"+maskID, r.ttl,
		func(p *datastore.PortalLink) bool { return p.UID != "" },
		func() (*datastore.PortalLink, error) { return r.inner.FindPortalLinkByMaskId(ctx, maskID) })
}

func (r *CachedPortalLinkRepository) UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *datastore.UpdatePortalLinkRequest) (*datastore.PortalLink, error) {
	result, err := r.inner.UpdatePortalLink(ctx, projectID, portalLink, request)
	if err == nil && portalLink.TokenMaskId != "" {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "portal_links_by_mask:"+portalLink.TokenMaskId)
	}
	return result, err
}

func (r *CachedPortalLinkRepository) CreatePortalLink(ctx context.Context, projectID string, req *datastore.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	return r.inner.CreatePortalLink(ctx, projectID, req)
}
func (r *CachedPortalLinkRepository) GetPortalLink(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	return r.inner.GetPortalLink(ctx, projectID, portalLinkID)
}
func (r *CachedPortalLinkRepository) GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	return r.inner.GetPortalLinkByToken(ctx, token)
}
func (r *CachedPortalLinkRepository) GetPortalLinkByOwnerID(ctx context.Context, projectID, ownerID string) (*datastore.PortalLink, error) {
	return r.inner.GetPortalLinkByOwnerID(ctx, projectID, ownerID)
}
func (r *CachedPortalLinkRepository) RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	return r.inner.RefreshPortalLinkAuthToken(ctx, projectID, portalLinkID)
}
func (r *CachedPortalLinkRepository) RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error {
	return r.inner.RevokePortalLink(ctx, projectID, portalLinkID)
}
func (r *CachedPortalLinkRepository) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	return r.inner.LoadPortalLinksPaged(ctx, projectID, filter, pageable)
}
func (r *CachedPortalLinkRepository) FindPortalLinksByOwnerID(ctx context.Context, ownerID string) ([]datastore.PortalLink, error) {
	return r.inner.FindPortalLinksByOwnerID(ctx, ownerID)
}

// ============================================================================
// OrganisationRepository
// ============================================================================

type CachedOrganisationRepository struct {
	inner  datastore.OrganisationRepository
	cache  cachedrepo.Cache
	ttl    time.Duration
	logger cachedrepo.Logger
}

func NewCachedOrganisationRepository(inner datastore.OrganisationRepository, c cachedrepo.Cache, ttl time.Duration, logger cachedrepo.Logger) *CachedOrganisationRepository {
	return &CachedOrganisationRepository{inner: inner, cache: c, ttl: ttl, logger: logger}
}

func (r *CachedOrganisationRepository) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	return cachedrepo.FetchOne(ctx, r.cache, r.logger, "organisations:"+id, r.ttl,
		func(o *datastore.Organisation) bool { return o.UID != "" },
		func() (*datastore.Organisation, error) { return r.inner.FetchOrganisationByID(ctx, id) })
}

func (r *CachedOrganisationRepository) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	err := r.inner.UpdateOrganisation(ctx, org)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "organisations:"+org.UID)
	}
	return err
}

func (r *CachedOrganisationRepository) UpdateOrganisationLicenseData(ctx context.Context, orgID, licenseData string) error {
	err := r.inner.UpdateOrganisationLicenseData(ctx, orgID, licenseData)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "organisations:"+orgID)
	}
	return err
}

func (r *CachedOrganisationRepository) DeleteOrganisation(ctx context.Context, id string) error {
	err := r.inner.DeleteOrganisation(ctx, id)
	if err == nil {
		cachedrepo.Invalidate(ctx, r.cache, r.logger, "organisations:"+id)
	}
	return err
}

func (r *CachedOrganisationRepository) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	return r.inner.CreateOrganisation(ctx, org)
}
func (r *CachedOrganisationRepository) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	return r.inner.FetchOrganisationByCustomDomain(ctx, domain)
}
func (r *CachedOrganisationRepository) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	return r.inner.FetchOrganisationByAssignedDomain(ctx, domain)
}
func (r *CachedOrganisationRepository) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	return r.inner.LoadOrganisationsPaged(ctx, pageable)
}
func (r *CachedOrganisationRepository) LoadOrganisationsPagedWithSearch(ctx context.Context, pageable datastore.Pageable, search string) ([]datastore.Organisation, datastore.PaginationData, error) {
	return r.inner.LoadOrganisationsPagedWithSearch(ctx, pageable, search)
}
func (r *CachedOrganisationRepository) CountOrganisations(ctx context.Context) (int64, error) {
	return r.inner.CountOrganisations(ctx)
}
func (r *CachedOrganisationRepository) CalculateUsage(ctx context.Context, orgID string, startTime, endTime time.Time) (*datastore.OrganisationUsage, error) {
	return r.inner.CalculateUsage(ctx, orgID, startTime, endTime)
}
