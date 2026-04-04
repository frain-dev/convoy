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

func TestCachedEndpointRepository_FindEndpointByID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123", ProjectID: "proj-1", Url: "https://example.com"}

	mockCache.EXPECT().Get(gomock.Any(), "endpoints:proj-1:ep-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			e := data.(*datastore.Endpoint)
			*e = *endpoint
			return nil
		})

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointByID(context.Background(), "ep-123", "proj-1")

	require.NoError(t, err)
	require.Equal(t, "ep-123", result.UID)
}

func TestCachedEndpointRepository_FindEndpointByID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123", Url: "https://example.com"}

	mockCache.EXPECT().Get(gomock.Any(), "endpoints:proj-1:ep-123", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindEndpointByID(gomock.Any(), "ep-123", "proj-1").Return(endpoint, nil)
	mockCache.EXPECT().Set(gomock.Any(), "endpoints:proj-1:ep-123", endpoint, 2*time.Minute).Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointByID(context.Background(), "ep-123", "proj-1")

	require.NoError(t, err)
	require.Equal(t, "ep-123", result.UID)
}

func TestCachedEndpointRepository_FindEndpointByID_CacheError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123"}

	mockCache.EXPECT().Get(gomock.Any(), "endpoints:proj-1:ep-123", gomock.Any()).Return(errors.New("redis down"))
	mockRepo.EXPECT().FindEndpointByID(gomock.Any(), "ep-123", "proj-1").Return(endpoint, nil)
	mockCache.EXPECT().Set(gomock.Any(), "endpoints:proj-1:ep-123", endpoint, 2*time.Minute).Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointByID(context.Background(), "ep-123", "proj-1")

	require.NoError(t, err)
	require.Equal(t, "ep-123", result.UID)
}

func TestCachedEndpointRepository_FindEndpointsByOwnerID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	eps := []datastore.Endpoint{
		{UID: "ep-1", OwnerID: "owner-1"},
		{UID: "ep-2", OwnerID: "owner-1"},
	}

	mockCache.EXPECT().Get(gomock.Any(), "endpoints_by_owner:proj-1:owner-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			ce := data.(*cachedEndpoints)
			ce.Endpoints = eps
			return nil
		})

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointsByOwnerID(context.Background(), "proj-1", "owner-1")

	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestCachedEndpointRepository_FindEndpointsByOwnerID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	eps := []datastore.Endpoint{{UID: "ep-1", OwnerID: "owner-1"}}

	mockCache.EXPECT().Get(gomock.Any(), "endpoints_by_owner:proj-1:owner-1", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindEndpointsByOwnerID(gomock.Any(), "proj-1", "owner-1").Return(eps, nil)
	mockCache.EXPECT().Set(gomock.Any(), "endpoints_by_owner:proj-1:owner-1", gomock.Any(), 2*time.Minute).Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindEndpointsByOwnerID(context.Background(), "proj-1", "owner-1")

	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestCachedEndpointRepository_UpdateEndpoint_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123", OwnerID: "owner-1"}

	mockRepo.EXPECT().UpdateEndpoint(gomock.Any(), endpoint, "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints_by_owner:proj-1:owner-1").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.UpdateEndpoint(context.Background(), endpoint, "proj-1")

	require.NoError(t, err)
}

func TestCachedEndpointRepository_UpdateEndpointStatus_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockRepo.EXPECT().UpdateEndpointStatus(gomock.Any(), "proj-1", "ep-123", datastore.InactiveEndpointStatus).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.UpdateEndpointStatus(context.Background(), "proj-1", "ep-123", datastore.InactiveEndpointStatus)

	require.NoError(t, err)
}

func TestCachedEndpointRepository_UpdateSecrets_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockRepo.EXPECT().UpdateSecrets(gomock.Any(), "ep-123", "proj-1", gomock.Any()).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.UpdateSecrets(context.Background(), "ep-123", "proj-1", datastore.Secrets{})

	require.NoError(t, err)
}

func TestCachedEndpointRepository_DeleteEndpoint_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123", OwnerID: "owner-1"}

	mockRepo.EXPECT().DeleteEndpoint(gomock.Any(), endpoint, "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints_by_owner:proj-1:owner-1").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.DeleteEndpoint(context.Background(), endpoint, "proj-1")

	require.NoError(t, err)
}

func TestCachedEndpointRepository_DeleteSecret_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-123"}

	mockRepo.EXPECT().DeleteSecret(gomock.Any(), endpoint, "secret-1", "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints:proj-1:ep-123").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.DeleteSecret(context.Background(), endpoint, "secret-1", "proj-1")

	require.NoError(t, err)
}

func TestCachedEndpointRepository_CreateEndpoint_InvalidatesOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockEndpointRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	endpoint := &datastore.Endpoint{UID: "ep-new", OwnerID: "owner-1"}

	mockRepo.EXPECT().CreateEndpoint(gomock.Any(), endpoint, "proj-1").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "endpoints_by_owner:proj-1:owner-1").Return(nil)

	repo := NewCachedEndpointRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.CreateEndpoint(context.Background(), endpoint, "proj-1")

	require.NoError(t, err)
}
