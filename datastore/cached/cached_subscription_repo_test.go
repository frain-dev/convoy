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

func TestCachedSubscriptionRepository_FindByEndpointID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	subs := []datastore.Subscription{
		{UID: "sub-1", EndpointID: "ep-1"},
		{UID: "sub-2", EndpointID: "ep-1"},
	}

	mockCache.EXPECT().Get(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			cs := data.(*cachedSubscriptions)
			cs.Subscriptions = subs
			return nil
		})

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	result, err := repo.FindSubscriptionsByEndpointID(context.Background(), "proj-1", "ep-1")

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "sub-1", result[0].UID)
}

func TestCachedSubscriptionRepository_FindByEndpointID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	subs := []datastore.Subscription{
		{UID: "sub-1", EndpointID: "ep-1"},
	}

	mockCache.EXPECT().Get(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "proj-1", "ep-1").Return(subs, nil)
	mockCache.EXPECT().Set(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any(), 30*time.Second).Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	result, err := repo.FindSubscriptionsByEndpointID(context.Background(), "proj-1", "ep-1")

	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestCachedSubscriptionRepository_FindByEndpointID_CacheError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	subs := []datastore.Subscription{{UID: "sub-1"}}

	mockCache.EXPECT().Get(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any()).Return(errors.New("redis down"))
	mockRepo.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "proj-1", "ep-1").Return(subs, nil)
	mockCache.EXPECT().Set(gomock.Any(), "subs_by_endpoint:proj-1:ep-1", gomock.Any(), 30*time.Second).Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	result, err := repo.FindSubscriptionsByEndpointID(context.Background(), "proj-1", "ep-1")

	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestCachedSubscriptionRepository_CreateSubscription_Invalidates(t *testing.T) {
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
	err := repo.CreateSubscription(context.Background(), "proj-1", sub)

	require.NoError(t, err)
}

func TestCachedSubscriptionRepository_UpdateSubscription_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	sub := &datastore.Subscription{UID: "sub-1", EndpointID: "ep-1"}

	mockRepo.EXPECT().UpdateSubscription(gomock.Any(), "proj-1", sub).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "subs_by_endpoint:proj-1:ep-1").Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	err := repo.UpdateSubscription(context.Background(), "proj-1", sub)

	require.NoError(t, err)
}

func TestCachedSubscriptionRepository_DeleteSubscription_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	sub := &datastore.Subscription{UID: "sub-1", EndpointID: "ep-1"}

	mockRepo.EXPECT().DeleteSubscription(gomock.Any(), "proj-1", sub).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "subs_by_endpoint:proj-1:ep-1").Return(nil)

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	err := repo.DeleteSubscription(context.Background(), "proj-1", sub)

	require.NoError(t, err)
}

func TestCachedSubscriptionRepository_EmptyEndpointID_SkipsInvalidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)

	sub := &datastore.Subscription{UID: "sub-1", EndpointID: ""}

	mockRepo.EXPECT().CreateSubscription(gomock.Any(), "proj-1", sub).Return(nil)
	// No cache.Delete expected since EndpointID is empty

	repo := NewCachedSubscriptionRepository(mockRepo, mockCache, 30*time.Second, logger)
	err := repo.CreateSubscription(context.Background(), "proj-1", sub)

	require.NoError(t, err)
}
