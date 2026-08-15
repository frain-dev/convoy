package handlers

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

func newRepoAccessorHandler(t *testing.T, withCache bool) *Handler {
	t.Helper()

	ctrl := gomock.NewController(t)
	db := mocks.NewMockDatabase(ctrl)
	db.EXPECT().GetConn().Return(nil).AnyTimes()
	db.EXPECT().GetHook().Return(nil).AnyTimes()

	a := &types.APIOptions{DB: db, Logger: logger.New("test", slog.LevelError)}
	if withCache {
		a.Cache = mocks.NewMockCache(ctrl)
	}

	return &Handler{A: a}
}

// Regression: subscription writes through the API must invalidate
// "subs_by_endpoint:<project>:<endpoint>", which only happens when the handlers
// hold the cache-invalidating repository. The cached repository used to be built
// solely by the dataplane worker, so its invalidation never ran for an API or
// dashboard edit and the worker matched on the pre-change list until the TTL.
func TestSubscriptionRepo_InvalidatesWhenCacheAvailable(t *testing.T) {
	h := newRepoAccessorHandler(t, true)

	require.IsType(t, &cached.CachedSubscriptionRepository{}, h.subscriptionRepo(),
		"subscription writes must go through the cache-invalidating repository")
}

func TestSubscriptionRepo_PlainRepoWithoutCache(t *testing.T) {
	h := newRepoAccessorHandler(t, false)

	_, isCached := h.subscriptionRepo().(*cached.CachedSubscriptionRepository)
	require.False(t, isCached, "no cache available, so the plain repository is wired")
}

// Filter mutations share the same contract: the worker matches on
// "filters:<subscription>:<eventType>", so a create, update or delete has to
// evict it.
func TestFilterWriteRepo_InvalidatesWhenCacheAvailable(t *testing.T) {
	h := newRepoAccessorHandler(t, true)

	require.IsType(t, &cached.CachedFilterRepository{}, h.filterWriteRepo(),
		"filter writes must go through the cache-invalidating repository")
}

func TestFilterWriteRepo_PlainRepoWithoutCache(t *testing.T) {
	h := newRepoAccessorHandler(t, false)

	_, isCached := h.filterWriteRepo().(*cached.CachedFilterRepository)
	require.False(t, isCached, "no cache available, so the plain repository is wired")
}
