package api_keys

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		panic(err)
	}
	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	// Load config again and ensure it's set properly
	err = config.LoadConfig("")
	require.NoError(t, err)

	_, err = config.Get()
	require.NoError(t, err)

	// Initialize KeyManager
	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	return db, ctx
}

func createAPIKeyService(t *testing.T, db database.Database) *Service {
	t.Helper()
	return New(log.NewLogger(os.Stdout), db)
}

func seedTestData(t *testing.T, db database.Database) (*datastore.User, *datastore.Organisation, *datastore.Project) {
	ctx := context.Background()

	// Create user
	userRepo := postgres.NewUserRepo(db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create organisation
	logger := log.NewLogger(os.Stdout)
	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create project
	projectRepo := postgres.NewProjectRepo(db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err = projectRepo.CreateProject(ctx, project)
	require.NoError(t, err)

	return user, org, project
}

// ============================================================================
// CreateAPIKey Tests
// ============================================================================

func TestCreateAPIKey_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Test API Key",
		Type:   datastore.ProjectKey,
		MaskID: "test_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "test_hash",
		Salt:      "test_salt",
		UserID:    user.UID,
		ExpiresAt: null.Time{},
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify the key was created
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, apiKey.Name, fetched.Name)
	require.Equal(t, apiKey.Type, fetched.Type)
	require.Equal(t, apiKey.MaskID, fetched.MaskID)
	require.Equal(t, apiKey.Role.Type, fetched.Role.Type)
	require.Equal(t, apiKey.Role.Project, fetched.Role.Project)
}

func TestCreateAPIKey_PersonalKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Personal API Key",
		Type:   datastore.PersonalKey,
		MaskID: "personal_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "personal_hash",
		Salt:      "personal_salt",
		UserID:    user.UID,
		ExpiresAt: null.NewTime(time.Now().Add(30*24*time.Hour), true),
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify the key was created
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.PersonalKey, fetched.Type)
	require.Equal(t, user.UID, fetched.UserID)
	require.True(t, fetched.ExpiresAt.Valid)
}

func TestCreateAPIKey_ProjectKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Project API Key",
		Type:   datastore.ProjectKey,
		MaskID: "project_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "project_hash",
		Salt:   "project_salt",
		UserID: "", // Project keys don't have user_id
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify the key was created
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.ProjectKey, fetched.Type)
	require.Empty(t, fetched.UserID)
}

func TestCreateAPIKey_WithExpiration(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days from now

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Expiring API Key",
		Type:   datastore.PersonalKey,
		MaskID: "expiring_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:      "expiring_hash",
		Salt:      "expiring_salt",
		UserID:    user.UID,
		ExpiresAt: null.NewTime(expiresAt, true),
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify expiration date
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.True(t, fetched.ExpiresAt.Valid)
	require.WithinDuration(t, expiresAt, fetched.ExpiresAt.Time, time.Second)
}

func TestCreateAPIKey_RoleAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Admin API Key",
		Type:   datastore.PersonalKey,
		MaskID: "admin_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "admin_hash",
		Salt:   "admin_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify role
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, fetched.Role.Type)
	require.Equal(t, project.UID, fetched.Role.Project)
}

func TestCreateAPIKey_RoleViewer(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Viewer API Key",
		Type:   datastore.PersonalKey,
		MaskID: "viewer_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectViewer,
			Project: project.UID,
		},
		Hash:   "viewer_hash",
		Salt:   "viewer_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify role
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectViewer, fetched.Role.Type)
}

func TestCreateAPIKey_WithEndpointRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)

	// Create an endpoint
	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Test Endpoint",
		Url:       "https://example.com/webhook",
		Secrets: []datastore.Secret{
			{Value: "test-secret"},
		},
	}
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Endpoint API Key",
		Type:   datastore.CLIKey,
		MaskID: "endpoint_mask_123",
		Role: auth.Role{
			Type:     auth.RoleAPI,
			Project:  project.UID,
			Endpoint: endpoint.UID,
		},
		Hash:   "endpoint_hash",
		Salt:   "endpoint_salt",
		UserID: user.UID,
	}

	err = service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify endpoint role
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, endpoint.UID, fetched.Role.Endpoint)
}

func TestCreateAPIKey_NilAPIKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createAPIKeyService(t, db)

	err := service.CreateAPIKey(ctx, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestCreateAPIKey_VerifyHash(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	expectedHash := "very_secure_hash_value"

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Hash Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "hash_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   expectedHash,
		Salt:   "test_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify hash is stored correctly
	fetched, err := service.GetAPIKeyByHash(ctx, expectedHash)
	require.NoError(t, err)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, expectedHash, fetched.Hash)
}

func TestCreateAPIKey_VerifySalt(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	expectedSalt := "unique_salt_value"

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Salt Test Key",
		Type:   datastore.PersonalKey,
		MaskID: "salt_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "test_hash",
		Salt:   expectedSalt,
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify salt is stored correctly
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, expectedSalt, fetched.Salt)
}

func TestCreateAPIKey_VerifyMaskID(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	expectedMaskID := "unique_mask_id_12345"

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "MaskID Test Key",
		Type:   datastore.PersonalKey,
		MaskID: expectedMaskID,
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

	// Verify mask_id is stored correctly and can be retrieved
	fetched, err := service.GetAPIKeyByMaskID(ctx, expectedMaskID)
	require.NoError(t, err)
	require.Equal(t, apiKey.UID, fetched.UID)
	require.Equal(t, expectedMaskID, fetched.MaskID)
}

func TestCreateAPIKey_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Persistence Test Key",
		Type:   datastore.ProjectKey,
		MaskID: "persist_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "persist_hash",
		Salt:   "persist_salt",
		UserID: user.UID,
	}

	// Create the key
	err := service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Verify all fields are persisted correctly
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
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

func TestCreateAPIKey_CLIKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "CLI API Key",
		Type:   datastore.CLIKey,
		MaskID: "cli_mask_123",
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   "cli_hash",
		Salt:   "cli_salt",
		UserID: user.UID,
	}

	err := service.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)

	// Verify the key type
	fetched, err := service.GetAPIKeyByID(ctx, apiKey.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.CLIKey, fetched.Type)
}
