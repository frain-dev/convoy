package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestCreateApiKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func generateApiKey() *datastore.APIKey {
	key := &datastore.APIKey{
		UID:    ulid.Make().String(),
		MaskID: ulid.Make().String(),
		Name:   "Test Api Key",
		Type:   datastore.ProjectKey,
		Role: auth.Role{
			Type:     auth.RoleAdmin,
			Project:  ulid.Make().String(),
			Endpoint: ulid.Make().String(),
		},
		Hash:      ulid.Make().String(),
		Salt:      ulid.Make().String(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	return key
}
