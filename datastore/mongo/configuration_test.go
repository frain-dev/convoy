//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_CreateConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	configRepo := NewConfigRepo(store)
	config := generateConfig()

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	require.Equal(t, config.UID, newConfig.UID)
	require.Equal(t, config.IsAnalyticsEnabled, newConfig.IsAnalyticsEnabled)
}

func Test_LoadConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	configRepo := NewConfigRepo(store)
	config := generateConfig()

	_, err := configRepo.LoadConfiguration(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrConfigNotFound))

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	require.Equal(t, config.UID, newConfig.UID)
}

func Test_UpdateConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	configRepo := NewConfigRepo(store)
	config := generateConfig()

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	config.IsAnalyticsEnabled = false
	require.NoError(t, configRepo.UpdateConfiguration(context.Background(), config))

	newConfig, err := configRepo.LoadConfiguration(context.Background())
	require.NoError(t, err)

	require.Equal(t, config.UID, newConfig.UID)
	require.Equal(t, config.IsAnalyticsEnabled, newConfig.IsAnalyticsEnabled)
}

func generateConfig() *datastore.Configuration {
	return &datastore.Configuration{
		UID:                uuid.NewString(),
		IsAnalyticsEnabled: true,
	}
}
