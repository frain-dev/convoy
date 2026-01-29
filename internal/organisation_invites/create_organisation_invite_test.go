package organisation_invites

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
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

func seedUser(t *testing.T, db database.Database) *datastore.User {
	t.Helper()

	userRepo := postgres.NewUserRepo(db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}

	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}

func seedOrganisation(t *testing.T, db database.Database) *datastore.Organisation {
	t.Helper()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Organisation",
		OwnerID: user.UID,
	}

	orgRepo := organisations.New(log.NewLogger(os.Stdout), db)
	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func seedProject(t *testing.T, db database.Database, org *datastore.Organisation) *datastore.Project {
	t.Helper()

	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	return project
}

func seedEndpoint(t *testing.T, db database.Database, project *datastore.Project) *datastore.Endpoint {
	t.Helper()

	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		Name:               "Test Endpoint",
		Url:                "https://example.com/webhook",
		SupportEmail:       "support@example.com",
		OwnerID:            project.UID,
		RateLimit:          5000,
		RateLimitDuration:  60000, // 1 minute in milliseconds
		HttpTimeout:        30000, // 30 seconds in milliseconds
		AdvancedSignatures: false,
		Secrets: []datastore.Secret{
			{Value: "test-secret"},
		},
	}

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}

func seedOrganisationInvite(t *testing.T, db database.Database, org *datastore.Organisation, status datastore.InviteStatus) *datastore.OrganisationInvite {
	t.Helper()

	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   fmt.Sprintf("invitee-%s@example.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleOrganisationAdmin,
			Project: "",
		},
		Status:    status,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(context.Background(), invite)
	require.NoError(t, err)

	return invite
}

func TestCreateOrganisationInvite_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "invitee@example.com",
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleOrganisationAdmin,
			Project: "",
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify invite was created
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, invite.UID, fetched.UID)
	require.Equal(t, invite.OrganisationID, fetched.OrganisationID)
	require.Equal(t, invite.InviteeEmail, fetched.InviteeEmail)
	require.Equal(t, invite.Token, fetched.Token)
	require.Equal(t, invite.Status, fetched.Status)
	require.Equal(t, invite.Role.Type, fetched.Role.Type)
}

func TestCreateOrganisationInvite_WithProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "project.invitee@example.com",
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify invite was created with project role
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, fetched.Role.Type)
	require.Equal(t, project.UID, fetched.Role.Project)
}

func TestCreateOrganisationInvite_WithEndpointRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	endpoint := seedEndpoint(t, db, project)
	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "endpoint.invitee@example.com",
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:     auth.RoleAPI,
			Endpoint: endpoint.UID,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify invite was created with endpoint role
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleAPI, fetched.Role.Type)
	require.Equal(t, endpoint.UID, fetched.Role.Endpoint)
}

func TestCreateOrganisationInvite_WithAllRoleFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	endpoint := seedEndpoint(t, db, project)
	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "full.role@example.com",
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:     auth.RoleProjectAdmin,
			Project:  project.UID,
			Endpoint: endpoint.UID,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify all role fields
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, fetched.Role.Type)
	require.Equal(t, project.UID, fetched.Role.Project)
	require.Equal(t, endpoint.UID, fetched.Role.Endpoint)
}

func TestCreateOrganisationInvite_DifferentStatuses(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	service := New(log.NewLogger(os.Stdout), db)

	statuses := []datastore.InviteStatus{
		datastore.InviteStatusPending,
		datastore.InviteStatusAccepted,
		datastore.InviteStatusDeclined,
		datastore.InviteStatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			invite := &datastore.OrganisationInvite{
				UID:            ulid.Make().String(),
				OrganisationID: org.UID,
				InviteeEmail:   fmt.Sprintf("%s@example.com", status),
				Token:          ulid.Make().String(),
				Role: auth.Role{
					Type: auth.RoleOrganisationAdmin,
				},
				Status:    status,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}

			err := service.CreateOrganisationInvite(ctx, invite)
			require.NoError(t, err)

			fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
			require.NoError(t, err)
			require.Equal(t, status, fetched.Status)
		})
	}
}

func TestCreateOrganisationInvite_NilInvite(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.CreateOrganisationInvite(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation invite cannot be nil")
}

func TestCreateOrganisationInvite_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	service := New(log.NewLogger(os.Stdout), db)

	invite := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "persistence@example.com",
		Token:          ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Create invite
	err := service.CreateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Create a new service instance to ensure no caching
	newService := New(log.NewLogger(os.Stdout), db)

	// Fetch and verify all fields match
	fetched, err := newService.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, invite.UID, fetched.UID)
	require.Equal(t, invite.OrganisationID, fetched.OrganisationID)
	require.Equal(t, invite.InviteeEmail, fetched.InviteeEmail)
	require.Equal(t, invite.Token, fetched.Token)
	require.Equal(t, invite.Status, fetched.Status)
	require.Equal(t, invite.Role.Type, fetched.Role.Type)
	require.Equal(t, invite.Role.Project, fetched.Role.Project)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)
	require.NotZero(t, fetched.ExpiresAt)
}

func TestCreateOrganisationInvite_UniqueToken(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	org := seedOrganisation(t, db)
	service := New(log.NewLogger(os.Stdout), db)

	token := ulid.Make().String()

	// Create first invite
	invite1 := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "user1@example.com",
		Token:          token,
		Role: auth.Role{
			Type: auth.RoleOrganisationAdmin,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := service.CreateOrganisationInvite(ctx, invite1)
	require.NoError(t, err)

	// Try to create second invite with same token - should fail
	invite2 := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		InviteeEmail:   "user2@example.com",
		Token:          token, // Same token
		Role: auth.Role{
			Type: auth.RoleOrganisationAdmin,
		},
		Status:    datastore.InviteStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = service.CreateOrganisationInvite(ctx, invite2)
	require.Error(t, err)
}
