//go:build integration
// +build integration

package api

import (
	"context"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRetentionCfg_GetRetentionPolicy(t *testing.T) {
	var db = getDB()
	testdb.PurgeDB(t, db)

	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	defaultPolicy := "24h"
	projectID := project.UID
	organisationID := org.UID

	retentionCfg := exporter.NewRetentionCfg(db, defaultPolicy, projectID, organisationID)

	ctx := context.Background()

	instanceDefaultsRepo := postgres.NewInstanceDefaultsRepo(db)
	instanceOverridesRepo := postgres.NewInstanceOverridesRepo(db)

	t.Run("Default Found", func(t *testing.T) {
		_, err := instanceDefaultsRepo.Create(ctx, &datastore.InstanceDefaults{
			UID:          "default2",
			ScopeType:    instance.OrganisationScope,
			Key:          instance.KeyRetentionPolicy,
			DefaultValue: "{\"policy\": \"36h\", \"enabled\": false}",
		})
		assert.NoError(t, err)

		// Fetch the retention policy
		retentionPolicy, err := retentionCfg.GetRetentionPolicy(ctx)
		assert.NoError(t, err)
		d, err := time.ParseDuration("36h")
		require.NoError(t, err)
		assert.Equal(t, d, retentionPolicy)
	})

	t.Run("Override Found", func(t *testing.T) {
		_, err := instanceOverridesRepo.Create(ctx, &datastore.InstanceOverrides{
			UID:       "override2",
			ScopeType: instance.ProjectScope,
			ScopeID:   projectID,
			Key:       instance.KeyRetentionPolicy,
			Value:     "{\"policy\": \"48h\", \"enabled\": false}",
		})
		assert.NoError(t, err)

		retentionPolicy, err := retentionCfg.GetRetentionPolicy(ctx)
		assert.NoError(t, err)
		d, err := time.ParseDuration("48h")
		require.NoError(t, err)
		assert.Equal(t, d, retentionPolicy)
	})

	t.Run("Fallback to Default Policy", func(t *testing.T) {
		_, err := db.GetDB().ExecContext(ctx, `DELETE FROM instance_overrides WHERE key = $1`, instance.KeyRetentionPolicy)
		assert.NoError(t, err)
		_, err = db.GetDB().ExecContext(ctx, `DELETE FROM instance_defaults WHERE key = $1`, instance.KeyRetentionPolicy)
		assert.NoError(t, err)

		retentionPolicy, err := retentionCfg.GetRetentionPolicy(ctx)
		assert.NoError(t, err)
		d, err := time.ParseDuration(defaultPolicy)
		require.NoError(t, err)
		assert.Equal(t, d, retentionPolicy)
	})

	testdb.PurgeDB(t, db)
}
