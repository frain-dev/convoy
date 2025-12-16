package api_keys

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys/models"
)

// ============================================================================
// UpdateAPIKey Tests
// ============================================================================

func TestUpdateAPIKey_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Original Name",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Update the key
	apiKey.Name = "Updated Name"
	apiKey.Role.Type = auth.RoleProjectViewer

	err = service.UpdateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify update
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", fetched.Name)
	require.Equal(t, auth.RoleProjectViewer, fetched.Role.Type)
}

func TestUpdateAPIKey_ChangeName(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Original Name",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Update name only
	apiKey.Name = "New Key Name"
	err = service.UpdateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, "New Key Name", fetched.Name)
}

func TestUpdateAPIKey_ChangeRoleType(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key with admin role
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Change to viewer role
	apiKey.Role.Type = auth.RoleProjectViewer
	err = service.UpdateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectViewer, fetched.Role.Type)
}

func TestUpdateAPIKey_ChangeRoleProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, org, project1 := seedTestData(t, db)

	// Create a second project
	projectRepo := postgres.NewProjectRepo(db)
	projectConfig := datastore.DefaultProjectConfig
	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Second Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err := projectRepo.CreateProject(ctx, project2)
	require.NoError(t, err)

	service := createAPIKeyService(t, db)

	// Create API key for project1
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project1.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err = service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Change to project2
	apiKey.Role.Project = project2.UID
	err = service.UpdateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, project2.UID, fetched.Role.Project)
}

func TestUpdateAPIKey_NilAPIKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	err := service.UpdateAPIKey(ctx, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestUpdateAPIKey_VerifyUpdatedAt(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Original Name",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Get original updated_at
	original, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	originalUpdatedAt := original.UpdatedAt

	// Update the key
	apiKey.Name = "Updated Name"
	err = service.UpdateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify updated_at changed
	updated, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.True(t, updated.UpdatedAt.After(originalUpdatedAt) || updated.UpdatedAt.Equal(originalUpdatedAt))
}

// ============================================================================
// RevokeAPIKeys Tests
// ============================================================================

func TestRevokeAPIKeys_SingleKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Revoke the key
	err = service.RevokeAPIKeys(ctx, []string{apiKey.UID})

	require.NoError(t, err)

	// Verify key cannot be fetched
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestRevokeAPIKeys_MultipleKeys(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create multiple API keys
	key1 := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Key 1",
		Type:   datastore.PersonalKey,
		MaskID: "mask_1",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "hash_1",
		Salt:   "salt_1",
		UserID: user.UID,
	}

	key2 := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Key 2",
		Type:   datastore.PersonalKey,
		MaskID: "mask_2",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "hash_2",
		Salt:   "salt_2",
		UserID: user.UID,
	}

	key3 := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Key 3",
		Type:   datastore.PersonalKey,
		MaskID: "mask_3",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "hash_3",
		Salt:   "salt_3",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, key1)
	require.NoError(t, err)
	err = service.CreateAPIKey(ctx, key2)
	require.NoError(t, err)
	err = service.CreateAPIKey(ctx, key3)
	require.NoError(t, err)

	// Revoke key1 and key2
	err = service.RevokeAPIKeys(ctx, []string{key1.UID, key2.UID})

	require.NoError(t, err)

	// Verify key1 and key2 cannot be fetched
	_, err = service.GetAPIKeyByID(ctx, key1.UID)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)

	_, err = service.GetAPIKeyByID(ctx, key2.UID)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)

	// Verify key3 can still be fetched
	fetched, err := service.GetAPIKeyByID(ctx, key3.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, key3.UID, fetched.UID)
}

func TestRevokeAPIKeys_EmptyArray(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	// Revoke with empty array (should be no-op)
	err := service.RevokeAPIKeys(ctx, []string{})

	require.NoError(t, err)
}

func TestRevokeAPIKeys_NonExistentKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	nonExistentID := ulid.Make().String()

	// Revoke non-existent key (should not error)
	err := service.RevokeAPIKeys(ctx, []string{nonExistentID})

	require.NoError(t, err)
}

func TestRevokeAPIKeys_VerifySoftDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Revoke the key
	err = service.RevokeAPIKeys(ctx, []string{apiKey.UID})
	require.NoError(t, err)

	// Verify key is not returned by GetAPIKeyByID (filters deleted keys)
	_, err = service.GetAPIKeyByID(ctx, apiKey.UID)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)

	// IMPORTANT: GetAPIKeyByMaskID IS special - it returns revoked keys with DeletedAt populated
	// This is needed for authentication flow to distinguish "not found" from "revoked"
	maskIDKey, err := service.GetAPIKeyByMaskID(ctx, apiKey.MaskID)
	require.NoError(t, err, "GetAPIKeyByMaskID should return revoked keys")
	require.NotNil(t, maskIDKey)
	require.False(t, maskIDKey.DeletedAt.IsZero(), "DeletedAt should be populated for revoked key")

	// Verify key is not returned by GetAPIKeyByHash (filters deleted keys)
	_, err = service.GetAPIKeyByHash(ctx, apiKey.Hash)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestRevokeAPIKeys_NotReturnedInPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 3 API keys
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   "Test Key",
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)

		// Revoke the first key
		if i == 0 {
			err = service.RevokeAPIKeys(ctx, []string{apiKey.UID})
			require.NoError(t, err)
		}
	}

	// Fetch paginated keys
	filter := &models.ApiKeyFilter{
		ProjectID: project.UID,
	}
	pageable := datastore.Pageable{
		PerPage: 10,
	}

	keys, _, err := service.LoadAPIKeysPaged(ctx, filter, &pageable)

	require.NoError(t, err)
	// Should only return 2 keys (excluding the revoked one)
	require.Len(t, keys, 2)
}
