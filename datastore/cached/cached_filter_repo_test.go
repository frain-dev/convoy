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

func TestCachedFilterRepository_FindBySubAndEventType_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	filter := &datastore.EventTypeFilter{UID: "f-1", SubscriptionID: "sub-1", EventType: "user.created"}

	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:user.created", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			cf := data.(*cachedFilter)
			cf.Filter = filter
			cf.Found = true
			return nil
		})

	// inner repo should NOT be called
	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "user.created")

	require.NoError(t, err)
	require.Equal(t, "f-1", result.UID)
}

func TestCachedFilterRepository_FindBySubAndEventType_CacheMiss(t *testing.T) {
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

func TestCachedFilterRepository_FindBySubAndEventType_CachesNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Cache miss
	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:*", gomock.Any()).Return(nil)

	// DB returns not found
	mockRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "sub-1", "*").
		Return(nil, datastore.ErrFilterNotFound)

	// Should cache the not-found result
	mockCache.EXPECT().Set(gomock.Any(), "filters:sub-1:*", gomock.Any(), 2*time.Minute).Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "*")

	require.Error(t, err)
	require.Nil(t, result)
}

func TestCachedFilterRepository_FindBySubAndEventType_ReturnsCachedNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Cache hit with Found=true but Filter=nil (cached not-found)
	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:*", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			cf := data.(*cachedFilter)
			cf.Filter = nil
			cf.Found = true
			return nil
		})

	// inner repo should NOT be called -- we return the cached nil
	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "*")

	require.NoError(t, err)
	require.Nil(t, result)
}

func TestCachedFilterRepository_FindBySubAndEventType_CacheError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	filter := &datastore.EventTypeFilter{UID: "f-1"}

	mockCache.EXPECT().Get(gomock.Any(), "filters:sub-1:user.created", gomock.Any()).Return(errors.New("redis down"))
	mockRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "sub-1", "user.created").Return(filter, nil)
	mockCache.EXPECT().Set(gomock.Any(), "filters:sub-1:user.created", gomock.Any(), 2*time.Minute).Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	result, err := repo.FindFilterBySubscriptionAndEventType(context.Background(), "sub-1", "user.created")

	require.NoError(t, err)
	require.Equal(t, "f-1", result.UID)
}

func TestCachedFilterRepository_CreateFilter_Invalidates(t *testing.T) {
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
	err := repo.CreateFilter(context.Background(), filter)

	require.NoError(t, err)
}

func TestCachedFilterRepository_UpdateFilter_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockFilterRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	filter := &datastore.EventTypeFilter{UID: "f-1", SubscriptionID: "sub-1", EventType: "user.created"}

	mockRepo.EXPECT().UpdateFilter(gomock.Any(), filter).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "filters:sub-1:user.created").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "filters:sub-1:*").Return(nil)

	repo := NewCachedFilterRepository(mockRepo, mockCache, 2*time.Minute, logger)
	err := repo.UpdateFilter(context.Background(), filter)

	require.NoError(t, err)
}
