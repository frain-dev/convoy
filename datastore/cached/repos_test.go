package cached

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

// ============================================================================
// ProjectRepository Tests
// ============================================================================

func TestCachedProjectRepo_FetchProjectByID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123", Name: "test"}
	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			*data.(*datastore.Project) = *project
			return nil
		})

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")
	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
}

func TestCachedProjectRepo_FetchProjectByID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123"}
	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(project, nil)
	mockCache.EXPECT().Set(gomock.Any(), "projects:proj-123", project, 5*time.Minute).Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")
	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
}

func TestCachedProjectRepo_UpdateProject_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123"}
	mockRepo.EXPECT().UpdateProject(gomock.Any(), project).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "projects:proj-123").Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	require.NoError(t, repo.UpdateProject(context.Background(), project))
}

func TestCachedProjectRepo_DeleteProject_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockRepo.EXPECT().DeleteProject(gomock.Any(), "proj-123").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "projects:proj-123").Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	require.NoError(t, repo.DeleteProject(context.Background(), "proj-123"))
}

// ============================================================================
// EndpointRepository Tests
// ============================================================================

func TestCachedEndpointRepo_FindEndpointByID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	ep := &datastore.Endpoint{UID: "ep-123", Url: "https://example.com"}
	mockCache.EXPECT().Get(gomock.Any(), "endpoints:proj-1:ep-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			*data.(*datastore.Endpoint) = *ep
			return nil
		})

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointByID(context.Background(), "ep-123", "proj-1")
	require.NoError(t, err)
	require.Equal(t, "ep-123", result.UID)
}

func TestCachedEndpointRepo_FindEndpointByID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	ep := &datastore.Endpoint{UID: "ep-123"}
	mockCache.EXPECT().Get(gomock.Any(), "endpoints:proj-1:ep-123", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindEndpointByID(gomock.Any(), "ep-123", "proj-1").Return(ep, nil)
	mockCache.EXPECT().Set(gomock.Any(), "endpoints:proj-1:ep-123", ep, 2*time.Minute).Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointByID(context.Background(), "ep-123", "proj-1")
	require.NoError(t, err)
	require.Equal(t, "ep-123", result.UID)
}

func TestCachedEndpointRepo_UpdateEndpoint_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	ep := &datastore.Endpoint{UID: "ep-123", OwnerID: "owner-1"}
	mockRepo.EXPECT().UpdateEndpoint(gomock.Any(), ep, "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints_by_owner:proj-1:owner-1").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	require.NoError(t, repo.UpdateEndpoint(context.Background(), ep, "proj-1"))
}

func TestCachedEndpointRepo_CreateEndpoint_InvalidatesOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	ep := &datastore.Endpoint{UID: "ep-new", OwnerID: "owner-1"}
	mockRepo.EXPECT().CreateEndpoint(gomock.Any(), ep, "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints_by_owner:proj-1:owner-1").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	require.NoError(t, repo.CreateEndpoint(context.Background(), ep, "proj-1"))
}

// ============================================================================
// SubscriptionRepository Tests
// ============================================================================

func TestCachedSubRepo_FindByEndpointID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	subs := []datastore.Subscription{{UID: "sub-1"}}
	mockCache.EXPECT().Get(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "proj-1", "ep-1").Return(subs, nil)
	mockCache.EXPECT().Set(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any(), 30*time.Second).Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	result, err := repo.FindSubscriptionsByEndpointID(context.Background(), "proj-1", "ep-1")
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestCachedSubRepo_CreateSubscription_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	sub := &datastore.Subscription{UID: "sub-1", EndpointID: "ep-1"}
	mockRepo.EXPECT().CreateSubscription(gomock.Any(), "proj-1", sub).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "subs_by_endpoint:proj-1:ep-1").Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	require.NoError(t, repo.CreateSubscription(context.Background(), "proj-1", sub))
}

func TestCachedSubRepo_EmptyEndpointID_SkipsInvalidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)

	sub := &datastore.Subscription{UID: "sub-1", EndpointID: ""}
	mockRepo.EXPECT().CreateSubscription(gomock.Any(), "proj-1", sub).Return(nil)
	// No Delete expected

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	require.NoError(t, repo.CreateSubscription(context.Background(), "proj-1", sub))
}

// ============================================================================
// FilterRepository Tests
// ============================================================================

func TestCachedFilterRepo_FindBySubAndEventType_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	filter := &datastore.EventTypeFilter{UID: "f-1", SubscriptionID: "sub-1", EventType: "user.created"}
	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:user.created", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "sub-1", "user.created").Return(filter, nil)
	mockCache.EXPECT().Set(gomock.Any(), "filters:sub-1:user.created", gomock.Any(), 2*time.Minute).Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "user.created")
	require.NoError(t, err)
	require.Equal(t, "f-1", result.UID)
}

func TestCachedFilterRepo_CachesNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:*", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "sub-1", "*").
		Return(nil, datastore.ErrFilterNotFound)
	mockCache.EXPECT().Set(gomock.Any(), "filters:sub-1:*", gomock.Any(), 2*time.Minute).Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "*")
	require.Error(t, err)
	require.Nil(t, result)
}

func TestCachedFilterRepo_CreateFilter_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	filter := &datastore.EventTypeFilter{UID: "f-1", SubscriptionID: "sub-1", EventType: "user.created"}
	mockRepo.EXPECT().CreateFilter(gomock.Any(), filter).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "filters:sub-1:user.created").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "filters:sub-1:*").Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	require.NoError(t, repo.CreateFilter(context.Background(), filter))
}

// ============================================================================
// APIKeyRepository Tests
// ============================================================================

func TestCachedAPIKeyRepo_GetByMaskID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockAPIKeyRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	apiKey := &datastore.APIKey{UID: "ak-123", MaskID: "mask-1"}
	mockCache.EXPECT().Get(gomock.Any(), "apikeys_by_mask:mask-1", gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), "mask-1").Return(apiKey, nil)
	mockCache.EXPECT().Set(gomock.Any(), "apikeys_by_mask:mask-1", apiKey, 5*time.Minute).Return(nil)

	repo := NewCachedAPIKeyRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.GetAPIKeyByMaskID(context.Background(), "mask-1")
	require.NoError(t, err)
	require.Equal(t, "ak-123", result.UID)
}

// ============================================================================
// PortalLinkRepository Tests
// ============================================================================

func TestCachedPortalLinkRepo_FindByMaskId_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockPortalLinkRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	pLink := &datastore.PortalLink{UID: "pl-123", TokenMaskId: "mask-1"}
	mockCache.EXPECT().Get(gomock.Any(), "portal_links_by_mask:mask-1", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindPortalLinkByMaskId(gomock.Any(), "mask-1").Return(pLink, nil)
	mockCache.EXPECT().Set(gomock.Any(), "portal_links_by_mask:mask-1", pLink, 5*time.Minute).Return(nil)

	repo := NewCachedPortalLinkRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FindPortalLinkByMaskId(context.Background(), "mask-1")
	require.NoError(t, err)
	require.Equal(t, "pl-123", result.UID)
}

// ============================================================================
// OrganisationRepository Tests
// ============================================================================

func TestCachedOrgRepo_FetchByID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	org := &datastore.Organisation{UID: "org-123"}
	mockCache.EXPECT().Get(gomock.Any(), "organisations:org-123", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FetchOrganisationByID(gomock.Any(), "org-123").Return(org, nil)
	mockCache.EXPECT().Set(gomock.Any(), "organisations:org-123", org, 5*time.Minute).Return(nil)

	repo := NewCachedOrganisationRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchOrganisationByID(context.Background(), "org-123")
	require.NoError(t, err)
	require.Equal(t, "org-123", result.UID)
}

func TestCachedOrgRepo_UpdateOrg_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	org := &datastore.Organisation{UID: "org-123"}
	mockRepo.EXPECT().UpdateOrganisation(gomock.Any(), org).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "organisations:org-123").Return(nil)

	repo := NewCachedOrganisationRepository(mockRepo, mockCache, 5*time.Minute, logger)
	require.NoError(t, repo.UpdateOrganisation(context.Background(), org))
}

// ============================================================================
// Cache Error Graceful Degradation
// ============================================================================

func TestCachedRepo_CacheError_FallsThrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123"}
	mockCache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("redis down"))
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(project, nil)
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")
	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
}

func TestCachedRepo_DBError_NotCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockCache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(nil, errors.New("db error"))

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")
	require.Error(t, err)
	require.Nil(t, result)
}
