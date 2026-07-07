package api

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/logger"
)

func TestEnsureAPIRepositories_setsReposWhenNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	lo := logger.New("test", slog.LevelError)

	a := &types.APIOptions{
		DB:     db,
		Logger: lo,
	}

	ensureAPIRepositories(a)

	require.NotNil(t, a.OrgRepo, "OrgRepo must be set when DB and Logger are present")
	require.NotNil(t, a.OrgMemberRepo, "OrgMemberRepo must be set when DB and Logger are present")
	require.NotNil(t, a.ProjectRepo, "ProjectRepo must be set when DB and Logger are present")
}

func TestEnsureAPIRepositories_preservesExplicitRepos(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	explicitOrg := mocks.NewMockOrganisationRepository(ctrl)
	lo := logger.New("test", slog.LevelError)

	a := &types.APIOptions{
		DB:      db,
		Logger:  lo,
		OrgRepo: explicitOrg,
	}

	ensureAPIRepositories(a)

	require.Same(t, explicitOrg, a.OrgRepo)
	require.NotNil(t, a.OrgMemberRepo)
	require.NotNil(t, a.ProjectRepo)
}

// Regression: project config updates via the API must invalidate the
// "projects:<id>" cache key. That only holds when the wired ProjectRepo is the
// cache-invalidating repository, since handlers write through it.
func TestEnsureAPIRepositories_wrapsProjectRepoInCacheWhenAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	lo := logger.New("test", slog.LevelError)

	a := &types.APIOptions{
		DB:     db,
		Logger: lo,
		Cache:  mocks.NewMockCache(ctrl),
	}

	ensureAPIRepositories(a)

	require.IsType(t, &cached.CachedProjectRepository{}, a.ProjectRepo,
		"ProjectRepo must be the cache-invalidating repository when a cache is available")
}

func TestEnsureAPIRepositories_plainProjectRepoWithoutCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	lo := logger.New("test", slog.LevelError)

	a := &types.APIOptions{
		DB:     db,
		Logger: lo,
	}

	ensureAPIRepositories(a)

	require.NotNil(t, a.ProjectRepo)
	_, isCached := a.ProjectRepo.(*cached.CachedProjectRepository)
	require.False(t, isCached, "no cache available, so the plain repository is wired")
}

func TestEnsureAPIRepositories_noOpWithoutDB(t *testing.T) {
	lo := logger.New("test", slog.LevelError)
	a := &types.APIOptions{Logger: lo}

	ensureAPIRepositories(a)

	require.Nil(t, a.OrgRepo)
	require.Nil(t, a.OrgMemberRepo)
	require.Nil(t, a.ProjectRepo)
}
