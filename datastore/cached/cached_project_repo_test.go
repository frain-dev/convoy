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

func TestCachedProjectRepository_FetchProjectByID_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123", Name: "test"}

	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, data interface{}) error {
			p := data.(*datastore.Project)
			*p = *project
			return nil
		})

	// inner repo should NOT be called on cache hit
	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")

	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
	require.Equal(t, "test", result.Name)
}

func TestCachedProjectRepository_FetchProjectByID_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123", Name: "test"}

	// Cache miss: returns nil error but data is zero-valued
	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).Return(nil)

	// Should fall through to DB
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(project, nil)

	// Should populate cache
	mockCache.EXPECT().Set(gomock.Any(), "projects:proj-123", project, 5*time.Minute).Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")

	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
}

func TestCachedProjectRepository_FetchProjectByID_CacheError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123", Name: "test"}

	// Cache returns error -- should degrade gracefully
	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).Return(errors.New("redis down"))
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(project, nil)
	mockCache.EXPECT().Set(gomock.Any(), "projects:proj-123", project, 5*time.Minute).Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")

	require.NoError(t, err)
	require.Equal(t, "proj-123", result.UID)
}

func TestCachedProjectRepository_FetchProjectByID_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockCache.EXPECT().Get(gomock.Any(), "projects:proj-123", gomock.Any()).Return(nil)
	mockRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-123").Return(nil, errors.New("db error"))

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	result, err := repo.FetchProjectByID(context.Background(), "proj-123")

	require.Error(t, err)
	require.Nil(t, result)
}

func TestCachedProjectRepository_UpdateProject_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	project := &datastore.Project{UID: "proj-123", Name: "updated"}

	mockRepo.EXPECT().UpdateProject(gomock.Any(), project).Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "projects:proj-123").Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	err := repo.UpdateProject(context.Background(), project)

	require.NoError(t, err)
}

func TestCachedProjectRepository_DeleteProject_Invalidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockRepo.EXPECT().DeleteProject(gomock.Any(), "proj-123").Return(nil)
	mockCache.EXPECT().Delete(gomock.Any(), "projects:proj-123").Return(nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)
	err := repo.DeleteProject(context.Background(), "proj-123")

	require.NoError(t, err)
}

func TestCachedProjectRepository_Passthrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProjectRepository(ctrl)
	mockCache := mocks.NewMockCache(ctrl)
	logger := mocks.NewMockLogger(ctrl)

	project := &datastore.Project{UID: "proj-123"}

	mockRepo.EXPECT().CreateProject(gomock.Any(), project).Return(nil)
	mockRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(5), nil)

	repo := NewCachedProjectRepository(mockRepo, mockCache, 5*time.Minute, logger)

	err := repo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	count, err := repo.CountProjects(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(5), count)
}
