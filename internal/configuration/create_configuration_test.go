package configuration

import (
	"context"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	if testEnv == nil {
		t.Fatal("testEnv is nil - TestMain may not have run successfully")
	}

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	db := postgres.NewFromConnection(conn)

	return db, ctx
}

func seedConfiguration(t *testing.T, db database.Database, storageType datastore.StorageType) *datastore.Configuration {
	t.Helper()

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: storageType,
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "720h",
			IsRetentionPolicyEnabled: true,
		},
	}

	if storageType == datastore.S3 {
		cfg.StoragePolicy.S3 = &datastore.S3Storage{
			Bucket:    null.StringFrom("test-bucket"),
			AccessKey: null.StringFrom("test-access-key"),
			SecretKey: null.StringFrom("test-secret-key"),
			Region:    null.StringFrom("us-east-1"),
			Prefix:    null.StringFrom("convoy/"),
			Endpoint:  null.StringFrom("https://s3.amazonaws.com"),
		}
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: null.NewString("", false),
		}
	} else {
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: null.StringFrom("/var/convoy/storage"),
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
	}

	service := New(log.NewLogger(os.Stdout), db)
	err := service.CreateConfiguration(context.Background(), cfg)
	require.NoError(t, err)

	return cfg
}

func TestCreateConfiguration_WithS3Storage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    false,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &datastore.S3Storage{
				Bucket:       null.StringFrom("my-bucket"),
				AccessKey:    null.StringFrom("AKIAIOSFODNN7EXAMPLE"),
				SecretKey:    null.StringFrom("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				Region:       null.StringFrom("us-west-2"),
				Prefix:       null.StringFrom("convoy-events/"),
				Endpoint:     null.StringFrom("https://s3.us-west-2.amazonaws.com"),
				SessionToken: null.StringFrom(""),
			},
			OnPrem: &datastore.OnPremStorage{
				Path: null.NewString("", false),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "168h",
			IsRetentionPolicyEnabled: true,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify configuration was created
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.UID, loaded.UID)
	require.Equal(t, cfg.IsAnalyticsEnabled, loaded.IsAnalyticsEnabled)
	require.Equal(t, cfg.IsSignupEnabled, loaded.IsSignupEnabled)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)
	require.Equal(t, "my-bucket", loaded.StoragePolicy.S3.Bucket.String)
	require.Equal(t, "us-west-2", loaded.StoragePolicy.S3.Region.String)
	require.Equal(t, "convoy-events/", loaded.StoragePolicy.S3.Prefix.String)
	require.Equal(t, "168h", loaded.RetentionPolicy.Policy)
	require.True(t, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}

func TestCreateConfiguration_WithOnPremStorage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: false,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{
				Path: null.StringFrom("/mnt/convoy-storage"),
			},
			S3: &datastore.S3Storage{
				Prefix:       null.NewString("", false),
				Bucket:       null.NewString("", false),
				AccessKey:    null.NewString("", false),
				SecretKey:    null.NewString("", false),
				Region:       null.NewString("", false),
				SessionToken: null.NewString("", false),
				Endpoint:     null.NewString("", false),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "720h",
			IsRetentionPolicyEnabled: false,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify configuration was created
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.UID, loaded.UID)
	require.Equal(t, cfg.IsAnalyticsEnabled, loaded.IsAnalyticsEnabled)
	require.Equal(t, cfg.IsSignupEnabled, loaded.IsSignupEnabled)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)
	require.Equal(t, "/mnt/convoy-storage", loaded.StoragePolicy.OnPrem.Path.String)
	require.True(t, loaded.StoragePolicy.OnPrem.Path.Valid)
	// Verify S3 fields are empty
	require.False(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.False(t, loaded.StoragePolicy.S3.AccessKey.Valid)
	require.Equal(t, "720h", loaded.RetentionPolicy.Policy)
	require.False(t, loaded.RetentionPolicy.IsRetentionPolicyEnabled)
}

func TestCreateConfiguration_WithMinimalFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: false,
		IsSignupEnabled:    false,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{
				Path: null.StringFrom("/tmp"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "",
			IsRetentionPolicyEnabled: false,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify configuration was created
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.UID, loaded.UID)
	require.False(t, loaded.IsAnalyticsEnabled)
	require.False(t, loaded.IsSignupEnabled)
}

func TestCreateConfiguration_NilConfiguration(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.CreateConfiguration(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration cannot be nil")
}

func TestCreateConfiguration_S3StorageNormalization(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &datastore.S3Storage{
				Bucket:    null.StringFrom("test-bucket"),
				AccessKey: null.StringFrom("key"),
				SecretKey: null.StringFrom("secret"),
				Region:    null.StringFrom("us-east-1"),
			},
			// OnPrem should be normalized to empty values
			OnPrem: &datastore.OnPremStorage{
				Path: null.StringFrom("/should/be/cleared"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "72h",
			IsRetentionPolicyEnabled: true,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify OnPrem was normalized (cleared) for S3 storage type
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.S3, loaded.StoragePolicy.Type)
	require.False(t, loaded.StoragePolicy.OnPrem.Path.Valid)
}

func TestCreateConfiguration_OnPremStorageNormalization(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{
				Path: null.StringFrom("/var/storage"),
			},
			// S3 should be normalized to empty values
			S3: &datastore.S3Storage{
				Bucket:    null.StringFrom("should-be-cleared"),
				AccessKey: null.StringFrom("key"),
				SecretKey: null.StringFrom("secret"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "72h",
			IsRetentionPolicyEnabled: true,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify S3 was normalized (cleared) for OnPrem storage type
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.Equal(t, datastore.OnPrem, loaded.StoragePolicy.Type)
	require.False(t, loaded.StoragePolicy.S3.Bucket.Valid)
	require.False(t, loaded.StoragePolicy.S3.AccessKey.Valid)
	require.False(t, loaded.StoragePolicy.S3.SecretKey.Valid)
}

func TestCreateConfiguration_VerifyTimestamps(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	cfg := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{
				Path: null.StringFrom("/tmp"),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "168h",
			IsRetentionPolicyEnabled: true,
		},
	}

	err := service.CreateConfiguration(ctx, cfg)
	require.NoError(t, err)

	// Verify timestamps are set
	loaded, err := service.LoadConfiguration(ctx)
	require.NoError(t, err)
	require.NotZero(t, loaded.CreatedAt)
	require.NotZero(t, loaded.UpdatedAt)
	require.False(t, loaded.DeletedAt.Valid) // Should not be deleted
}
