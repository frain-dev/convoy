package api_keys

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys/models"
)

func TestGetAPIKeyByID_Found(t *testing.T) {
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

	// Fetch by ID
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)

	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, apiKey.Name, fetched.Name)
}

func TestGetAPIKeyByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	nonExistentID := ulid.Make().String()

	fetched, err := service.GetAPIKeyByID(ctx, nonExistentID)

	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestGetAPIKeyByMaskID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	maskID := "unique_mask_id_12345"

	// Create API key
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test Key",
		Type:   datastore.PersonalKey,
		MaskID: maskID,
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

	// Fetch by MaskID (CRITICAL for authentication)
	fetched, err := service.GetAPIKeyByMaskID(ctx, maskID)

	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, maskID, fetched.MaskID)
}

func TestGetAPIKeyByMaskID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	nonExistentMaskID := "non_existent_mask"

	fetched, err := service.GetAPIKeyByMaskID(ctx, nonExistentMaskID)

	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestGetAPIKeyByHash_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	hash := "unique_hash_value_67890"

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
		Hash:   hash,
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Fetch by Hash
	fetched, err := service.GetAPIKeyByHash(ctx, hash)

	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, hash, fetched.Hash)
}

func TestGetAPIKeyByHash_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	nonExistentHash := "non_existent_hash"

	fetched, err := service.GetAPIKeyByHash(ctx, nonExistentHash)

	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestGetAPIKeyByProjectID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key with project role
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Project Key",
		Type:   datastore.ProjectKey,
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

	// Fetch by ProjectID
	fetched, err := service.GetAPIKeyByProjectID(ctx, project.UID)

	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, project.UID, fetched.Role.Project)
}

func TestGetAPIKeyByProjectID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	nonExistentProjectID := ulid.Make().String()

	fetched, err := service.GetAPIKeyByProjectID(ctx, nonExistentProjectID)

	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, models.ErrAPIKeyNotFound)
}

func TestGetAPIKey_VerifyAllFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create API key with all fields populated
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Complete Key",
		Type:   datastore.PersonalKey,
		MaskID: "complete_mask",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "complete_hash",
		Salt:   "complete_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Fetch and verify all fields
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)

	require.NoError(t, err)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, apiKey.Name, fetched.Name)
	require.Equal(t, apiKey.Type, fetched.Type)
	require.Equal(t, apiKey.MaskID, fetched.MaskID)
	require.Equal(t, apiKey.Hash, fetched.Hash)
	require.Equal(t, apiKey.Salt, fetched.Salt)
	require.Equal(t, apiKey.UserID, fetched.UserID)
	require.Equal(t, apiKey.Role.Type, fetched.Role.Type)
	require.Equal(t, apiKey.Role.Project, fetched.Role.Project)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)
}
