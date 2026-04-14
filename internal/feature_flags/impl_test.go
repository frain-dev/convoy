package feature_flags

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/users"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		panic(err)
	}
	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, *Service) {
	t.Helper()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	logger := log.New("convoy", log.LevelInfo)
	return db, New(logger, db)
}

func seedOrg(t *testing.T, db database.Database) *datastore.Organisation {
	t.Helper()
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}
	require.NoError(t, userRepo.CreateUser(ctx, user))

	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	require.NoError(t, orgRepo.CreateOrganisation(ctx, org))

	return org
}

// ============================================================================
// Feature Flag Tests
// ============================================================================

func TestLoadFeatureFlags(t *testing.T) {
	_, svc := setupTestDB(t)
	ctx := context.Background()

	// The feature_flags table is seeded by migrations, so it should have entries
	flags, err := svc.LoadFeatureFlags(ctx)
	require.NoError(t, err)
	require.NotNil(t, flags)
}

func TestFetchFeatureFlagByKey(t *testing.T) {
	_, svc := setupTestDB(t)
	ctx := context.Background()

	t.Run("existing flag", func(t *testing.T) {
		// circuit-breaker is a known seeded feature flag
		flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
		require.NoError(t, err)
		require.Equal(t, "circuit-breaker", flag.FeatureKey)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.FetchFeatureFlagByKey(ctx, "nonexistent-flag")
		require.Equal(t, datastore.ErrFeatureFlagNotFound, err)
	})
}

func TestFetchFeatureFlagByID(t *testing.T) {
	_, svc := setupTestDB(t)
	ctx := context.Background()

	// Fetch a known flag first to get its ID
	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	t.Run("existing flag", func(t *testing.T) {
		fetched, err := svc.FetchFeatureFlagByID(ctx, flag.UID)
		require.NoError(t, err)
		require.Equal(t, flag.UID, fetched.UID)
		require.Equal(t, "circuit-breaker", fetched.FeatureKey)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.FetchFeatureFlagByID(ctx, "nonexistent-id")
		require.Equal(t, datastore.ErrFeatureFlagNotFound, err)
	})
}

func TestUpdateFeatureFlag(t *testing.T) {
	_, svc := setupTestDB(t)
	ctx := context.Background()

	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	// Toggle enabled
	newEnabled := !flag.Enabled
	require.NoError(t, svc.UpdateFeatureFlag(ctx, flag.UID, newEnabled))

	updated, err := svc.FetchFeatureFlagByID(ctx, flag.UID)
	require.NoError(t, err)
	require.Equal(t, newEnabled, updated.Enabled)
}

// ============================================================================
// Feature Flag Override Tests
// ============================================================================

func TestUpsertAndFetchFeatureFlagOverride(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: flag.UID,
		OwnerType:     "organisation",
		OwnerID:       org.UID,
		Enabled:       true,
		EnabledBy:     null.StringFrom("test-user"),
	}

	require.NoError(t, svc.UpsertFeatureFlagOverride(ctx, override))

	fetched, err := svc.FetchFeatureFlagOverrideByOwner(ctx, "organisation", org.UID, flag.UID)
	require.NoError(t, err)
	require.Equal(t, true, fetched.Enabled)
	require.Equal(t, "organisation", fetched.OwnerType)
	require.Equal(t, org.UID, fetched.OwnerID)
}

func TestLoadFeatureFlagOverridesByOwner(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: flag.UID,
		OwnerType:     "organisation",
		OwnerID:       org.UID,
		Enabled:       true,
	}
	require.NoError(t, svc.UpsertFeatureFlagOverride(ctx, override))

	overrides, err := svc.LoadFeatureFlagOverridesByOwner(ctx, "organisation", org.UID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(overrides), 1)
}

func TestDeleteFeatureFlagOverride(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: flag.UID,
		OwnerType:     "organisation",
		OwnerID:       org.UID,
		Enabled:       true,
	}
	require.NoError(t, svc.UpsertFeatureFlagOverride(ctx, override))

	require.NoError(t, svc.DeleteFeatureFlagOverride(ctx, "organisation", org.UID, flag.UID))

	_, err = svc.FetchFeatureFlagOverrideByOwner(ctx, "organisation", org.UID, flag.UID)
	require.Equal(t, datastore.ErrFeatureFlagOverrideNotFound, err)
}

// ============================================================================
// Early Adopter Feature Tests
// ============================================================================

func TestUpsertAndGetEarlyAdopterFeature(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	feature := &datastore.EarlyAdopterFeature{
		OrganisationID: org.UID,
		FeatureKey:     "mtls",
		Enabled:        true,
		EnabledBy:      null.StringFrom("test-user"),
	}

	require.NoError(t, svc.UpsertEarlyAdopterFeature(ctx, feature))

	fetched, err := svc.GetEarlyAdopterFeature(ctx, org.UID, "mtls")
	require.NoError(t, err)
	require.Equal(t, true, fetched.Enabled)
	require.Equal(t, org.UID, fetched.OrganisationID)
	require.Equal(t, "mtls", fetched.FeatureKey)
}

func TestLoadEarlyAdopterFeaturesByOrg(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	require.NoError(t, svc.UpsertEarlyAdopterFeature(ctx, &datastore.EarlyAdopterFeature{
		OrganisationID: org.UID,
		FeatureKey:     "mtls",
		Enabled:        true,
	}))

	require.NoError(t, svc.UpsertEarlyAdopterFeature(ctx, &datastore.EarlyAdopterFeature{
		OrganisationID: org.UID,
		FeatureKey:     "oauth-token-exchange",
		Enabled:        false,
	}))

	features, err := svc.LoadEarlyAdopterFeaturesByOrg(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, 2, len(features))
}

func TestDeleteEarlyAdopterFeature(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	require.NoError(t, svc.UpsertEarlyAdopterFeature(ctx, &datastore.EarlyAdopterFeature{
		OrganisationID: org.UID,
		FeatureKey:     "mtls",
		Enabled:        true,
	}))

	require.NoError(t, svc.DeleteEarlyAdopterFeature(ctx, org.UID, "mtls"))

	_, err := svc.GetEarlyAdopterFeature(ctx, org.UID, "mtls")
	require.Equal(t, datastore.ErrEarlyAdopterFeatureNotFound, err)
}

// ============================================================================
// Interface Method Tests (fflag.FeatureFlagFetcher, fflag.EarlyAdopterFeatureFetcher)
// ============================================================================

func TestFetchFeatureFlag_Interface(t *testing.T) {
	_, svc := setupTestDB(t)
	ctx := context.Background()

	info, err := svc.FetchFeatureFlag(ctx, "circuit-breaker")
	require.NoError(t, err)
	require.NotEmpty(t, info.UID)

	_, err = svc.FetchFeatureFlag(ctx, "nonexistent")
	require.Equal(t, datastore.ErrFeatureFlagNotFound, err)
}

func TestFetchFeatureFlagOverride_Interface(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	flag, err := svc.FetchFeatureFlagByKey(ctx, "circuit-breaker")
	require.NoError(t, err)

	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: flag.UID,
		OwnerType:     "organisation",
		OwnerID:       org.UID,
		Enabled:       true,
	}
	require.NoError(t, svc.UpsertFeatureFlagOverride(ctx, override))

	info, err := svc.FetchFeatureFlagOverride(ctx, "organisation", org.UID, flag.UID)
	require.NoError(t, err)
	require.True(t, info.Enabled)
}

func TestFetchEarlyAdopterFeature_Interface(t *testing.T) {
	db, svc := setupTestDB(t)
	org := seedOrg(t, db)
	ctx := context.Background()

	require.NoError(t, svc.UpsertEarlyAdopterFeature(ctx, &datastore.EarlyAdopterFeature{
		OrganisationID: org.UID,
		FeatureKey:     "mtls",
		Enabled:        true,
	}))

	info, err := svc.FetchEarlyAdopterFeature(ctx, org.UID, "mtls")
	require.NoError(t, err)
	require.True(t, info.Enabled)

	_, err = svc.FetchEarlyAdopterFeature(ctx, org.UID, "nonexistent")
	require.Equal(t, datastore.ErrEarlyAdopterFeatureNotFound, err)
}
