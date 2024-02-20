// go:build integration
// go:build integration
//go:build integration
// +build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guregu/null/v5"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_CreateConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	configRepo := NewConfigRepo(db)
	config := generateConfig()

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	newConfig.CreatedAt = time.Time{}
	newConfig.UpdatedAt = time.Time{}

	config.CreatedAt = time.Time{}
	config.UpdatedAt = time.Time{}

	require.Equal(t, config, newConfig)
}

func Test_LoadConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	configRepo := NewConfigRepo(db)
	config := generateConfig()

	_, err := configRepo.LoadConfiguration(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrConfigNotFound))

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	newConfig.CreatedAt = time.Time{}
	newConfig.UpdatedAt = time.Time{}

	config.CreatedAt = time.Time{}
	config.UpdatedAt = time.Time{}

	require.Equal(t, config, newConfig)
}

func Test_UpdateConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	configRepo := NewConfigRepo(db)
	config := generateConfig()

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	config.IsAnalyticsEnabled = false
	require.NoError(t, configRepo.UpdateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	newConfig.CreatedAt = time.Time{}
	newConfig.UpdatedAt = time.Time{}

	config.CreatedAt = time.Time{}
	config.UpdatedAt = time.Time{}

	require.Equal(t, config, newConfig)
}

func generateConfig() *datastore.Configuration {
	return &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    false,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			S3: &datastore.S3Storage{
				Prefix:       null.NewString("random7", true),
				Bucket:       null.NewString("random1", true),
				AccessKey:    null.NewString("random2", true),
				SecretKey:    null.NewString("random3", true),
				Region:       null.NewString("random4", true),
				SessionToken: null.NewString("random5", true),
				Endpoint:     null.NewString("random6", true),
			},
			OnPrem: &datastore.OnPremStorage{
				Path: null.NewString("path", true),
			},
		},
	}
}
