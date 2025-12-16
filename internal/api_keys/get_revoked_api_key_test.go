package api_keys

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys/models"
)

// TestGetAPIKeyByMaskID_RevokedKey verifies that revoked keys are returned with DeletedAt populated
// This is critical for authentication flow to distinguish between "not found" and "revoked" errors
func TestGetAPIKeyByMaskID_RevokedKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	apiKeyService := createAPIKeyService(t, db)

	// Create test data
	_, _, project := seedTestData(t, db)

	// Create an API key
	maskID := "test_mask_" + ulid.Make().String()
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test API Key for Revocation",
		Type:   datastore.ProjectKey,
		MaskID: maskID,
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "test_hash_123",
		Salt:      "test_salt_456",
		UserID:    "",
		ExpiresAt: null.Time{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create the API key
	err := apiKeyService.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify we can get the key before revocation
	retrievedKey, err := apiKeyService.GetAPIKeyByMaskID(ctx, maskID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKey)
	require.Equal(t, apiKey.UID, retrievedKey.UID)
	require.True(t, retrievedKey.DeletedAt.IsZero(), "DeletedAt should be zero for active key")

	// Revoke the API key
	err = apiKeyService.RevokeAPIKeys(ctx, []string{apiKey.UID})
	require.NoError(t, err)

	// Try to get the revoked key by mask ID
	// CRITICAL: This should still return the key with DeletedAt populated
	// so that NativeRealm can return "api key has been revoked" error
	revokedKey, err := apiKeyService.GetAPIKeyByMaskID(ctx, maskID)
	require.NoError(t, err, "GetAPIKeyByMaskID should return revoked keys")
	require.NotNil(t, revokedKey)
	require.Equal(t, apiKey.UID, revokedKey.UID)
	require.False(t, revokedKey.DeletedAt.IsZero(), "DeletedAt should be populated for revoked key")
	require.True(t, revokedKey.DeletedAt.Valid, "DeletedAt should be valid for revoked key")
	require.WithinDuration(t, time.Now(), revokedKey.DeletedAt.Time, 5*time.Second, "DeletedAt should be recent")
}

// TestGetAPIKeyByMaskID_NonRevokedKey verifies that non-revoked keys have zero DeletedAt
func TestGetAPIKeyByMaskID_NonRevokedKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	apiKeyService := createAPIKeyService(t, db)

	// Create test data
	_, _, project := seedTestData(t, db)

	// Create an API key
	maskID := "test_mask_" + ulid.Make().String()
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Active API Key",
		Type:   datastore.ProjectKey,
		MaskID: maskID,
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "test_hash_789",
		Salt:      "test_salt_012",
		UserID:    "",
		ExpiresAt: null.Time{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create the API key
	err := apiKeyService.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Get the key and verify DeletedAt is zero
	retrievedKey, err := apiKeyService.GetAPIKeyByMaskID(ctx, maskID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKey)
	require.Equal(t, apiKey.UID, retrievedKey.UID)
	require.True(t, retrievedKey.DeletedAt.IsZero(), "DeletedAt should be zero for active key")
	require.False(t, retrievedKey.DeletedAt.Valid, "DeletedAt should not be valid for active key")
}

// TestGetAPIKeyByID_DoesNotReturnRevokedKeys verifies that other Get methods filter deleted keys
func TestGetAPIKeyByID_DoesNotReturnRevokedKeys(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	// Create test data
	_, _, project := seedTestData(t, db)

	// Create an API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test API Key for ID Filtering",
		Type:   datastore.ProjectKey,
		MaskID: "test_mask_" + ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "test_hash_345",
		Salt:      "test_salt_678",
		UserID:    "",
		ExpiresAt: null.Time{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create the API key
	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify we can get the key before revocation
	retrievedKey, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKey)
	require.Equal(t, apiKey.UID, retrievedKey.UID)

	// Revoke the API key
	err = service.RevokeAPIKeys(ctx, []string{apiKey.UID})
	require.NoError(t, err)

	// Try to get the revoked key by ID
	// This should return ErrAPIKeyNotFound because FindAPIKeyByID filters deleted keys
	_, err = service.GetAPIKeyByID(ctx, apiKey.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound, "GetAPIKeyByID should not return revoked keys")
}
