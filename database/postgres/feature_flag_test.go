//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

// seedFeatureFlag inserts a feature_flags row directly so override rows have a
// valid feature_flag_id to reference (FK with ON DELETE CASCADE).
func seedFeatureFlag(t *testing.T, db database.Database, key string, enabled bool) string {
	t.Helper()

	id := ulid.Make().String()
	_, err := db.GetDB().ExecContext(context.Background(),
		`INSERT INTO convoy.feature_flags (id, feature_key, enabled) VALUES ($1, $2, $3)`,
		id, key, enabled)
	require.NoError(t, err)
	return id
}

func TestAnyEnabledOverride(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	ctx := context.Background()
	flagID := seedFeatureFlag(t, db, "circuit-breaker-"+ulid.Make().String(), false)

	// No overrides yet.
	exists, err := AnyEnabledOverride(ctx, db, flagID)
	require.NoError(t, err)
	require.False(t, exists)

	// A disabled override must not count.
	require.NoError(t, UpsertFeatureFlagOverride(ctx, db, &datastore.FeatureFlagOverride{
		FeatureFlagID: flagID,
		OwnerType:     "organisation",
		OwnerID:       ulid.Make().String(),
		Enabled:       false,
	}))

	exists, err = AnyEnabledOverride(ctx, db, flagID)
	require.NoError(t, err)
	require.False(t, exists)

	// An enabled override for another owner flips it to true.
	require.NoError(t, UpsertFeatureFlagOverride(ctx, db, &datastore.FeatureFlagOverride{
		FeatureFlagID: flagID,
		OwnerType:     "organisation",
		OwnerID:       ulid.Make().String(),
		Enabled:       true,
	}))

	exists, err = AnyEnabledOverride(ctx, db, flagID)
	require.NoError(t, err)
	require.True(t, exists)

	// A different feature flag with no overrides is still false (scoping check).
	otherFlagID := seedFeatureFlag(t, db, "circuit-breaker-"+ulid.Make().String(), false)
	exists, err = AnyEnabledOverride(ctx, db, otherFlagID)
	require.NoError(t, err)
	require.False(t, exists)
}
