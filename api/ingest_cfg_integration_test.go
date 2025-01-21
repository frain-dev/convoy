//go:build integration
// +build integration

package api

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/cache"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIngestCfg_GetInstanceRateLimit(t *testing.T) {
	var db = getDB()
	var memoryCache cache.Cache = mcache.NewMemoryCache()

	testdb.PurgeDB(t, db)

	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	defaultRate := 100
	projectID := project.UID
	organisationID := org.UID

	ingestCfg := ingest.NewIngestCfg(db, memoryCache, defaultRate, projectID, organisationID, 3600)

	ctx := context.Background()

	instanceOverridesRepo := postgres.NewInstanceOverridesRepo(db)

	t.Run("Override Found", func(t *testing.T) {
		_, err := instanceOverridesRepo.Create(ctx, &datastore.InstanceOverrides{
			UID:       "override1",
			ScopeType: instance.ProjectScope,
			ScopeID:   projectID,
			Key:       instance.KeyInstanceIngestRate,
			Value:     "{\"value\": 200}",
		})
		assert.NoError(t, err)

		cacheKey := fmt.Sprintf("rate_limit:%s:%s:%s", instance.KeyInstanceIngestRate, projectID, organisationID)
		err = memoryCache.Delete(ctx, cacheKey)
		require.NoError(t, err)

		rateLimit, err := ingestCfg.GetInstanceRateLimitWithCache(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 200, rateLimit)
	})

	t.Run("Fallback to Default Rate", func(t *testing.T) {
		_, err := db.GetDB().ExecContext(ctx, `DELETE FROM convoy.instance_overrides WHERE key = $1`, instance.KeyInstanceIngestRate)
		assert.NoError(t, err)

		cacheKey := fmt.Sprintf("rate_limit:%s:%s:%s", instance.KeyInstanceIngestRate, projectID, organisationID)
		err = memoryCache.Delete(ctx, cacheKey)
		require.NoError(t, err)

		rateLimit, err := ingestCfg.GetInstanceRateLimitWithCache(ctx)
		assert.NoError(t, err)
		assert.Equal(t, defaultRate, rateLimit)
	})

	testdb.PurgeDB(t, db)
}
