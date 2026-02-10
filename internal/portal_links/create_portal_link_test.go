package portal_links

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/frain-dev/convoy/internal/users"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	testEnv *testenv.Environment
)

func SeedPortalLink(db database.Database, project *datastore.Project, ownerId string) (*datastore.PortalLink, error) {
	portalLink := &datastore.CreatePortalLinkRequest{
		Name:              fmt.Sprintf("TestPortalLink-%s", ulid.Make().String()),
		Endpoints:         []string{}, // Initialize as an empty slice instead of nil
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		OwnerID:           ownerId,
		CanManageEndpoint: true,
	}

	logger := log.NewLogger(os.Stdout)
	portalLinkRepo := New(logger, db)
	p, err := portalLinkRepo.CreatePortalLink(context.TODO(), project.UID, portalLink)
	if err != nil {
		return nil, err
	}

	return p, nil
}

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

func seedTestData(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()
	ctx := context.Background()
	logger := log.NewLogger(os.Stdout)

	// Create user
	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create organisation
	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create project
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
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

	return project
}

func seedEndpoint(t *testing.T, db database.Database, project *datastore.Project, ownerID string) *datastore.Endpoint {
	t.Helper()

	ctx := context.Background()
	endpointRepo := postgres.NewEndpointRepo(db)

	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   ownerID,
		Name:      "Test Endpoint",
		Status:    datastore.ActiveEndpointStatus,
		AppID:     ulid.Make().String(),
		Url:       "https://example.com/webhook",
		Secrets: []datastore.Secret{
			{Value: "test-secret"},
		},
	}

	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}

func TestCreatePortalLink_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.NotEmpty(t, portalLink.UID)
	require.Equal(t, request.Name, portalLink.Name)
	require.Equal(t, request.OwnerID, portalLink.OwnerID)
	require.Equal(t, project.UID, portalLink.ProjectID)
	require.Equal(t, datastore.PortalAuthType(request.AuthType), portalLink.AuthType)
	require.Equal(t, request.CanManageEndpoint, portalLink.CanManageEndpoint)
	require.NotNil(t, portalLink.Endpoints)
	require.Empty(t, portalLink.Endpoints)
}

func TestCreatePortalLink_WithEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Seed endpoints without owner_id
	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, endpoint2.UID},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, request.Name, portalLink.Name)
	require.Equal(t, request.OwnerID, portalLink.OwnerID)
	require.NotNil(t, portalLink.Endpoints)
	require.Equal(t, 2, len(portalLink.Endpoints))
	require.Contains(t, portalLink.Endpoints, endpoint1.UID)
	require.Contains(t, portalLink.Endpoints, endpoint2.UID)

	// Verify that endpoints have the correct owner_id
	endpointRepo := postgres.NewEndpointRepo(db)
	updatedEndpoint1, err := endpointRepo.FindEndpointByID(ctx, endpoint1.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, ownerID, updatedEndpoint1.OwnerID)

	updatedEndpoint2, err := endpointRepo.FindEndpointByID(ctx, endpoint2.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, ownerID, updatedEndpoint2.OwnerID)
}

func TestCreatePortalLink_WithEndpoints_AlreadyHaveOwnerID(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Seed endpoint that already has the same owner_id
	endpoint1 := seedEndpoint(t, db, project, ownerID)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Existing Owner",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, request.OwnerID, portalLink.OwnerID)
	require.Contains(t, portalLink.Endpoints, endpoint1.UID)
}

func TestCreatePortalLink_WithEndpoints_DifferentOwnerID_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	differentOwnerID := ulid.Make().String()

	// Seed endpoint that already has a different owner_id
	endpoint1 := seedEndpoint(t, db, project, differentOwnerID)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Different Owner",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "already has owner_id")
}

func TestCreatePortalLink_WithOwnerID_NoEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Seed endpoints with the same owner_id
	_ = seedEndpoint(t, db, project, ownerID)
	_ = seedEndpoint(t, db, project, ownerID)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Owner ID",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{}, // No endpoints provided
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, request.OwnerID, portalLink.OwnerID)
	require.NotNil(t, portalLink.Endpoints)
	require.Empty(t, portalLink.Endpoints) // Endpoints should be empty in response

	// Verify that auth_key is set when refresh token type is used
	require.NotEmpty(t, portalLink.AuthKey, "auth_key should be set for refresh token auth type")
	require.Contains(t, portalLink.AuthKey, "PRT.", "auth_key should have the correct prefix")

	// Verify portal link was created and endpoints are linked
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, 2, fetchedPortalLink.EndpointCount)
}

func TestCreatePortalLink_WithRefreshTokenAuthType(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Refresh Token Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, datastore.PortalAuthTypeRefreshToken, portalLink.AuthType)

	// Verify that auth_key is set when refresh token type is used
	require.NotEmpty(t, portalLink.AuthKey, "auth_key should be set for refresh token auth type")
	require.Contains(t, portalLink.AuthKey, "PRT.", "auth_key should have the correct prefix")
}

func TestCreatePortalLink_WithStaticTokenAuthType_NoAuthKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Static Token Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, datastore.PortalAuthTypeStaticToken, portalLink.AuthType)

	// Verify that auth_key is NOT set for static token auth type
	require.Empty(t, portalLink.AuthKey, "auth_key should not be set for static token auth type")
}

func TestCreatePortalLink_EmptyOwnerID_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link Auto Owner",
		OwnerID:           "", // Empty owner_id
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	// Should fail validation because owner_id is required
	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "owner")
}

func TestCreatePortalLink_InvalidRequest_MissingName(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "", // Missing name
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "name")
}

func TestCreatePortalLink_InvalidRequest_InvalidAuthType(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Invalid Auth Type Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          "invalid_auth_type", // Invalid auth type
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.Error(t, err)
	require.Nil(t, portalLink)
}

func TestCreatePortalLink_EndpointNotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Invalid Endpoint",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{"non-existent-endpoint-id"},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "failed to find endpoint")
}

func TestCreatePortalLink_MultipleEndpoints_SomeInvalid(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Seed one valid endpoint
	endpoint1 := seedEndpoint(t, db, project, "")

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link Mixed Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, "non-existent-endpoint-id"},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	// Should fail because one endpoint doesn't exist
	require.Error(t, err)
	require.Nil(t, portalLink)
}

func TestCreatePortalLink_VerifyTokenGenerated(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Token Verification Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)

	// Token is not returned in the response, but it should be generated in the database
	// Verify by fetching the portal link from the database
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, portalLink.UID, fetchedPortalLink.UID)
	require.NotEmpty(t, fetchedPortalLink.Token) // Token should exist in the database

	// Verify we can fetch it by token (checks that token was saved correctly)
	fetchedByToken, err := service.GetPortalLinkByToken(ctx, fetchedPortalLink.Token)
	require.NoError(t, err)
	require.Equal(t, portalLink.UID, fetchedByToken.UID)
}

func TestCreatePortalLink_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	request := &datastore.CreatePortalLinkRequest{
		Name:              "Persistence Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, request)

	require.NoError(t, err)
	require.NotNil(t, portalLink)

	// Fetch from database to verify persistence
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, portalLink.UID, fetchedPortalLink.UID)
	require.Equal(t, portalLink.Name, fetchedPortalLink.Name)
	require.Equal(t, portalLink.OwnerID, fetchedPortalLink.OwnerID)
	require.Equal(t, portalLink.AuthType, fetchedPortalLink.AuthType)
	require.Equal(t, portalLink.CanManageEndpoint, fetchedPortalLink.CanManageEndpoint)
}
