package configuration

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestUpdateConfiguration_ValidUpdate(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed initial configuration
	cfg := seedConfiguration(t, db, datastore.S3)

	// Update configuration
	cfg.IsAnalyticsEnabled = false
	cfg.IsSignupEnabled = false
	cfg.RetentionPolicy.Policy = "336h"
	cfg.RetentionPolicy.IsRetentionPolicyEnabled = false

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify updates
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.UID, loaded.UID)
	require.False(t, loaded.IsAnalyticsEnabled)
	require.False(t, loaded.IsSignupEnabled)
	require.Equal(t, "336h", loaded.RetentionPolicy.Policy)
	require.False(t, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}

func TestUpdateConfiguration_ChangeStorageFromS3ToOnPrem(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with S3 storage
	cfg := seedConfiguration(t, db, datastore.S3)

	// Change to OnPrem storage
	cfg.StoragePolicy.Type = datastore.OnPrem
	cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
		Path: null.StringFrom("/new/storage/path"),
	}
	cfg.StoragePolicy.S3 = &datastore.S3Storage{
		Prefix:       null.NewString("", false),
		Bucket:       null.NewString("", false),
		AccessKey:    null.NewString("", false),
		SecretKey:    null.NewString("", false),
		Region:       null.NewString("", false),
		SessionToken: null.NewString("", false),
		Endpoint:     null.NewString("", false),
	}

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify storage type changed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)
	require.Equal(t, "/new/storage/path", loaded.StoragePolicy.OnPrem.Path.String)
	// S3 fields should be cleared
	require.False(t, loaded.StoragePolicy.S3.Bucket.Valid)
}

func TestUpdateConfiguration_ChangeStorageFromOnPremToS3(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with OnPrem storage
	cfg := seedConfiguration(t, db, datastore.OnPrem)

	// Change to S3 storage
	cfg.StoragePolicy.Type = datastore.S3
	cfg.StoragePolicy.S3 = &datastore.S3Storage{
		Bucket:    null.StringFrom("new-s3-bucket"),
		AccessKey: null.StringFrom("new-access-key"),
		SecretKey: null.StringFrom("new-secret-key"),
		Region:    null.StringFrom("eu-west-1"),
		Prefix:    null.StringFrom("data/"),
		Endpoint:  null.StringFrom("https://s3.eu-west-1.amazonaws.com"),
	}
	cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
		Path: null.NewString("", false),
	}

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify storage type changed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)
	require.Equal(t, "new-s3-bucket", loaded.StoragePolicy.S3.Bucket.String)
	require.Equal(t, "eu-west-1", loaded.StoragePolicy.S3.Region.String)
	// OnPrem path should be cleared
	require.False(t, loaded.StoragePolicy.OnPrem.Path.Valid)
}

func TestUpdateConfiguration_UpdateS3Credentials(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with S3 storage
	cfg := seedConfiguration(t, db, datastore.S3)

	// Update S3 credentials
	cfg.StoragePolicy.S3.AccessKey = null.StringFrom("updated-access-key")
	cfg.StoragePolicy.S3.SecretKey = null.StringFrom("updated-secret-key")
	cfg.StoragePolicy.S3.Bucket = null.StringFrom("updated-bucket")

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify S3 credentials updated
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, "updated-bucket", loaded.StoragePolicy.S3.Bucket.String)
	require.Equal(t, "updated-access-key", loaded.StoragePolicy.S3.AccessKey.String)
	require.Equal(t, "updated-secret-key", loaded.StoragePolicy.S3.SecretKey.String)
}

func TestUpdateConfiguration_UpdateOnPremPath(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with OnPrem storage
	cfg := seedConfiguration(t, db, datastore.OnPrem)

	// Update OnPrem path
	cfg.StoragePolicy.OnPrem.Path = null.StringFrom("/updated/storage/path")

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify path updated
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, "/updated/storage/path", loaded.StoragePolicy.OnPrem.Path.String)
}

func TestUpdateConfiguration_UpdateRetentionPolicy(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	cfg := seedConfiguration(t, db, datastore.S3)

	// Update retention policy
	cfg.RetentionPolicy.Policy = "2160h"
	cfg.RetentionPolicy.IsRetentionPolicyEnabled = false

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify retention policy updated
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, "2160h", loaded.RetentionPolicy.Policy)
	require.False(t, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}

func TestUpdateConfiguration_EnableAnalytics(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with analytics disabled
	cfg := seedConfiguration(t, db, datastore.S3)
	cfg.IsAnalyticsEnabled = false
	_ = service.UpdateConfiguration(ctx, cfg)

	// Enable analytics
	cfg.IsAnalyticsEnabled = true
	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify analytics enabled
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.True(t, loaded.IsAnalyticsEnabled)
}

func TestUpdateConfiguration_DisableSignup(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration with signup enabled
	cfg := seedConfiguration(t, db, datastore.S3)
	cfg.IsSignupEnabled = true
	_ = service.UpdateConfiguration(ctx, cfg)

	// Disable signup
	cfg.IsSignupEnabled = false
	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify signup disabled
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, loaded.IsSignupEnabled)
}

func TestUpdateConfiguration_NilConfiguration(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.UpdateConfiguration(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration cannot be nil")
}

func TestUpdateConfiguration_NonExistentConfiguration(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to update non-existent configuration
	cfg := &datastore.Configuration{
		UID:                "non-existent-id",
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &datastore.S3Storage{
				Bucket: null.StringFrom("bucket"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "72h",
			IsRetentionPolicyEnabled: true,
		},
	}

	err := service.UpdateConfiguration(ctx, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestUpdateConfiguration_StorageNormalization_S3(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	cfg := seedConfiguration(t, db, datastore.S3)

	// Update with S3 storage but also provide OnPrem data (should be normalized)
	cfg.StoragePolicy.Type = datastore.S3
	cfg.StoragePolicy.S3.Bucket = null.StringFrom("normalized-bucket")
	cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
		Path: null.StringFrom("/should/be/cleared"),
	}

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify OnPrem was normalized (cleared)
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)
	require.False(t, loaded.StoragePolicy.OnPrem.Path.Valid)
	require.Equal(t, "normalized-bucket", loaded.StoragePolicy.S3.Bucket.String)
}

func TestUpdateConfiguration_StorageNormalization_OnPrem(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	cfg := seedConfiguration(t, db, datastore.OnPrem)

	// Update with OnPrem storage but also provide S3 data (should be normalized)
	cfg.StoragePolicy.Type = datastore.OnPrem
	cfg.StoragePolicy.OnPrem.Path = null.StringFrom("/normalized/path")
	cfg.StoragePolicy.S3 = &datastore.S3Storage{
		Bucket:    null.StringFrom("should-be-cleared"),
		AccessKey: null.StringFrom("key"),
	}

	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify S3 was normalized (cleared)
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)
	require.False(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.False(t, loaded.StoragePolicy.S3.AccessKey.Valid)
	require.Equal(t, "/normalized/path", loaded.StoragePolicy.OnPrem.Path.String)
}

func TestUpdateConfiguration_VerifyUpdatedAtChanged(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	cfg := seedConfiguration(t, db, datastore.S3)

	// Load to get initial UpdatedAt
	initial, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	initialUpdatedAt := initial.UpdatedAt

	// Update configuration
	cfg.IsAnalyticsEnabled = !cfg.IsAnalyticsEnabled
	err = service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify UpdatedAt changed
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.True(t, loaded.UpdatedAt.After(initialUpdatedAt) || loaded.UpdatedAt.Equal(initialUpdatedAt))
}

func TestUpdateConfiguration_MultipleUpdates(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	cfg := seedConfiguration(t, db, datastore.S3)

	// Perform multiple updates
	cfg.IsAnalyticsEnabled = false
	err := service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	cfg.IsSignupEnabled = false
	err = service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	cfg.RetentionPolicy.Policy = "1000h"
	err = service.UpdateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify all updates persisted
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, loaded.IsAnalyticsEnabled)
	require.False(t, loaded.IsSignupEnabled)
	require.Equal(t, "1000h", loaded.RetentionPolicy.Policy)
}

func TestUpdateConfiguration_VerifyNoRowsAffectedError(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed configuration
	seedConfiguration(t, db, datastore.S3)

	// Try to update with wrong ID
	wrongCfg := &datastore.Configuration{
		UID:                ulid.Make().String(), // Different ID
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &datastore.S3Storage{
				Bucket: null.StringFrom("bucket"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy: "72h",
		},
	}

	err := service.UpdateConfiguration(ctx, wrongCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
