package configuration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestLoadConfiguration_S3Storage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with S3 storage
	seeded := seedConfiguration(t, db, datastore.S3)

	// Load configuration
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, seeded.UID, loaded.UID)
	require.Equal(t, seeded.IsAnalyticsEnabled, loaded.IsAnalyticsEnabled)
	require.Equal(t, seeded.IsSignupEnabled, loaded.IsSignupEnabled)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)
	require.NotNil(t, loaded.StoragePolicy.S3)
	require.True(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.Equal(t, "test-bucket", loaded.StoragePolicy.S3.Bucket.String)
}

func TestLoadConfiguration_OnPremStorage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with OnPrem storage
	seeded := seedConfiguration(t, db, datastore.OnPrem)

	// Load configuration
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, seeded.UID, loaded.UID)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)
	require.NotNil(t, loaded.StoragePolicy.OnPrem)
	require.True(t, loaded.StoragePolicy.OnPrem.Path.Valid)
	require.Equal(t, "/var/convoy/storage", loaded.StoragePolicy.OnPrem.Path.String)
}

func TestLoadConfiguration_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to load configuration when none exists
	_, err := service.LoadConfiguration(ctx)
	require.Error(t, err)
	require.Equal(t, datastore.ErrConfigNotFound, err)
}

func TestLoadConfiguration_VerifyRetentionPolicy(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	seeded := seedConfiguration(t, db, datastore.S3)

	// Load and verify retention policy
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded.RetentionPolicy)
	require.Equal(t, seeded.RetentionPolicy.Policy, loaded.RetentionPolicy.Policy)
	require.Equal(t, seeded.RetentionPolicy.IsRetentionPolicyEnabled, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}

func TestLoadConfiguration_VerifyS3FieldsReconstructed(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with S3 storage
	seedConfiguration(t, db, datastore.S3)

	// Load and verify all S3 fields are properly reconstructed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)

	// Verify S3 structure is populated
	require.NotNil(t, loaded.StoragePolicy.S3)
	require.True(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.True(t, loaded.StoragePolicy.S3.AccessKey.Valid)
	require.True(t, loaded.StoragePolicy.S3.SecretKey.Valid)
	require.True(t, loaded.StoragePolicy.S3.Region.Valid)
	require.True(t, loaded.StoragePolicy.S3.Prefix.Valid)
	require.True(t, loaded.StoragePolicy.S3.Endpoint.Valid)

	// Verify OnPrem is empty struct (backward compatibility)
	require.NotNil(t, loaded.StoragePolicy.OnPrem)
	require.False(t, loaded.StoragePolicy.OnPrem.Path.Valid)
}

func TestLoadConfiguration_VerifyOnPremFieldsReconstructed(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with OnPrem storage
	seedConfiguration(t, db, datastore.OnPrem)

	// Load and verify OnPrem fields are properly reconstructed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)

	// Verify OnPrem structure is populated
	require.NotNil(t, loaded.StoragePolicy.OnPrem)
	require.True(t, loaded.StoragePolicy.OnPrem.Path.Valid)

	// Verify S3 is empty struct (backward compatibility)
	require.NotNil(t, loaded.StoragePolicy.S3)
	require.False(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.False(t, loaded.StoragePolicy.S3.AccessKey.Valid)
	require.False(t, loaded.StoragePolicy.S3.SecretKey.Valid)
}

func TestLoadConfiguration_VerifyBooleanConversion(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	seedConfiguration(t, db, datastore.S3)

	// Load and verify boolean fields are correctly converted
	// (is_analytics_enabled is stored as TEXT in DB, converted to bool)
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.True(t, loaded.IsAnalyticsEnabled) // Should be true from seed
	require.True(t, loaded.IsSignupEnabled)    // Should be true from seed
}

func TestLoadConfiguration_OnlyOneConfiguration(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed multiple configurations (only last one should be loadable due to LIMIT 1)
	cfg1 := seedConfiguration(t, db, datastore.S3)
	cfg2 := seedConfiguration(t, db, datastore.OnPrem)

	// Load configuration - should return one (most recent based on query)
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Should match one of the seeded configs
	isValidConfig := loaded.UID == cfg1.UID || loaded.UID == cfg2.UID
	require.True(t, isValidConfig, "Loaded config should match one of the seeded configs")
}

func TestLoadConfiguration_VerifyTimestamps(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	seedConfiguration(t, db, datastore.S3)

	// Load and verify timestamps
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotZero(t, loaded.CreatedAt)
	require.NotZero(t, loaded.UpdatedAt)
	require.False(t, loaded.DeletedAt.Valid)
}

func TestLoadConfiguration_CompleteDataIntegrity(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	seeded := seedConfiguration(t, db, datastore.S3)

	// Load configuration
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)

	// Verify complete data integrity
	require.Equal(t, seeded.UID, loaded.UID)
	require.Equal(t, seeded.IsAnalyticsEnabled, loaded.IsAnalyticsEnabled)
	require.Equal(t, seeded.IsSignupEnabled, loaded.IsSignupEnabled)
	require.Equal(t, seeded.StoragePolicy.Type, loaded.StoragePolicy.Type)
	require.Equal(t, seeded.StoragePolicy.S3.Bucket.String, loaded.StoragePolicy.S3.Bucket.String)
	require.Equal(t, seeded.StoragePolicy.S3.AccessKey.String, loaded.StoragePolicy.S3.AccessKey.String)
	require.Equal(t, seeded.StoragePolicy.S3.Region.String, loaded.StoragePolicy.S3.Region.String)
	require.Equal(t, seeded.RetentionPolicy.Policy, loaded.RetentionPolicy.Policy)
	require.Equal(t, seeded.RetentionPolicy.IsRetentionPolicyEnabled, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}
