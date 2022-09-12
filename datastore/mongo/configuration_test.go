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

	configCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.ConfigCollection)

	require.NoError(t, configRepo.CreateConfiguration(configCtx, config))

	newConfig, err := configRepo.LoadConfiguration(configCtx)
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

	configCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.ConfigCollection)
	_, err := configRepo.LoadConfiguration(configCtx)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrConfigNotFound))

	require.NoError(t, configRepo.CreateConfiguration(configCtx, config))

	newConfig, err := configRepo.LoadConfiguration(configCtx)
	require.NoError(t, err)

	require.Equal(t, config.UID, newConfig.UID)
}

func Test_UpdateConfiguration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	configRepo := NewConfigRepo(store)
	config := generateConfig()

	configCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.ConfigCollection)
	require.NoError(t, configRepo.CreateConfiguration(configCtx, config))

	config.IsAnalyticsEnabled = false
	require.NoError(t, configRepo.UpdateConfiguration(configCtx, config))

	newConfig, err := configRepo.LoadConfiguration(configCtx)
	require.NoError(t, err)

	require.Equal(t, config.UID, newConfig.UID)
	require.Equal(t, config.IsAnalyticsEnabled, newConfig.IsAnalyticsEnabled)
}

func generateConfig() *datastore.Configuration {
	return &datastore.Configuration{
		UID:                uuid.NewString(),
		IsAnalyticsEnabled: true,
		DocumentStatus:     datastore.ActiveDocumentStatus,
	}
}
