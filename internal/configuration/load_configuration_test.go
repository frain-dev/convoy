package configuration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestLoadConfiguration_S3Storage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

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

	service := New(log.New("convoy", log.LevelInfo), db)

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

func TestLoadConfiguration_AzureBlobStorage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

	seeded := seedConfiguration(t, db, datastore.AzureBlob)

	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, seeded.UID, loaded.UID)
	require.Equal(t, datastore.AzureBlob, loaded.StoragePolicy.Type)
	require.NotNil(t, loaded.StoragePolicy.AzureBlob)
	require.True(t, loaded.StoragePolicy.AzureBlob.AccountName.Valid)
	require.Equal(t, "testaccount", loaded.StoragePolicy.AzureBlob.AccountName.String)
	require.Equal(t, "test-container", loaded.StoragePolicy.AzureBlob.ContainerName.String)
	// S3 and OnPrem should be nil
	require.Nil(t, loaded.StoragePolicy.S3)
	require.Nil(t, loaded.StoragePolicy.OnPrem)
}

func TestLoadConfiguration_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

	// Try to load configuration when none exists
	_, err := service.LoadConfiguration(ctx)
	require.Error(t, err)
	require.Equal(t, datastore.ErrConfigNotFound, err)
}

func TestCompleteAdminManagedMigration(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)
	seeded := seedConfiguration(t, db, datastore.OnPrem)
	_, err := db.GetConn().Exec(
		ctx,
		"UPDATE convoy.configurations SET admin_managed = NULL, retention_enabled = NULL WHERE id = $1",
		seeded.UID,
	)
	require.NoError(t, err)

	legacy, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, legacy.AdminManagedKnown)
	require.False(t, legacy.RetentionPolicy.EnabledKnown)

	adminManaged, retentionEnabled, err := service.CompleteAdminManagedMigration(ctx, seeded.UID, false)
	require.NoError(t, err)
	require.False(t, adminManaged)
	require.False(t, retentionEnabled)
	adminManaged, retentionEnabled, err = service.CompleteAdminManagedMigration(ctx, seeded.UID, true)
	require.NoError(t, err)
	require.False(t, adminManaged)
	require.False(t, retentionEnabled)

	migrated, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, migrated.AdminManaged)
	require.True(t, migrated.AdminManagedKnown)
	require.False(t, migrated.RetentionPolicy.Enabled)
	require.True(t, migrated.RetentionPolicy.EnabledKnown)
	require.Equal(t, seeded.StoragePolicy, migrated.StoragePolicy)
	require.Equal(t, seeded.IsSignupEnabled, migrated.IsSignupEnabled)
	require.Equal(t, seeded.IsAnalyticsEnabled, migrated.IsAnalyticsEnabled)
}

func TestCompleteAdminManagedMigration_ReturnsExistingMode(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)
	seeded := seedConfiguration(t, db, datastore.OnPrem)

	adminManaged, retentionEnabled, err := service.CompleteAdminManagedMigration(ctx, seeded.UID, false)
	require.NoError(t, err)
	require.False(t, adminManaged)
	require.True(t, retentionEnabled)
}

func TestCompleteAdminManagedMigration_PreservesKnownRetention(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)
	seeded := seedConfiguration(t, db, datastore.OnPrem)
	_, err := db.GetConn().Exec(
		ctx,
		"UPDATE convoy.configurations SET admin_managed = NULL WHERE id = $1",
		seeded.UID,
	)
	require.NoError(t, err)

	adminManaged, retentionEnabled, err := service.CompleteAdminManagedMigration(ctx, seeded.UID, false)
	require.NoError(t, err)
	require.False(t, adminManaged)
	require.True(t, retentionEnabled)
}

func TestLoadConfiguration_VerifyRetentionPolicy(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

	// Seed configuration
	seeded := seedConfiguration(t, db, datastore.S3)

	// Load and verify retention policy
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded.RetentionPolicy)
	require.Equal(t, seeded.RetentionPolicy.Period, loaded.RetentionPolicy.Period)
	require.Equal(t, seeded.WebhookArchiving.Enabled, loaded.WebhookArchiving.Enabled)
}

func TestLoadConfiguration_VerifyS3FieldsReconstructed(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

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

	// OnPrem and Azure should be nil for S3 type
	require.Nil(t, loaded.StoragePolicy.OnPrem)
	require.Nil(t, loaded.StoragePolicy.AzureBlob)
}

func TestLoadConfiguration_VerifyOnPremFieldsReconstructed(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

	// Seed configuration with OnPrem storage
	seedConfiguration(t, db, datastore.OnPrem)

	// Load and verify OnPrem fields are properly reconstructed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)

	// Verify OnPrem structure is populated
	require.NotNil(t, loaded.StoragePolicy.OnPrem)
	require.True(t, loaded.StoragePolicy.OnPrem.Path.Valid)

	// S3 and Azure should be nil for OnPrem type
	require.Nil(t, loaded.StoragePolicy.S3)
	require.Nil(t, loaded.StoragePolicy.AzureBlob)
}

func TestLoadConfiguration_VerifyBooleanConversion(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

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

	service := New(log.New("convoy", log.LevelInfo), db)

	// Seed, soft-delete, seed again so the unique live-row index still holds.
	cfg1 := seedConfiguration(t, db, datastore.S3)
	_, err := db.GetConn().Exec(ctx, `UPDATE convoy.configurations SET deleted_at = NOW() WHERE id = $1`, cfg1.UID)
	require.NoError(t, err)
	cfg2 := seedConfiguration(t, db, datastore.OnPrem)

	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, cfg2.UID, loaded.UID)
}

func TestLoadConfiguration_VerifyTimestamps(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)

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

	service := New(log.New("convoy", log.LevelInfo), db)

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
	require.Equal(t, seeded.RetentionPolicy.Period, loaded.RetentionPolicy.Period)
	require.Equal(t, seeded.WebhookArchiving.Enabled, loaded.WebhookArchiving.Enabled)
}

func TestLoadConfiguration_RetentionEnabledNull(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.New("convoy", log.LevelInfo), db)
	seeded := seedConfiguration(t, db, datastore.S3)

	_, err := db.GetConn().Exec(ctx, `UPDATE convoy.configurations SET retention_enabled = NULL WHERE id = $1`, seeded.UID)
	require.NoError(t, err)

	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, loaded.RetentionPolicy.EnabledKnown)
	require.False(t, loaded.RetentionPolicy.Enabled)
	require.Equal(t, seeded.RetentionPolicy.Period, loaded.RetentionPolicy.Period)
}
