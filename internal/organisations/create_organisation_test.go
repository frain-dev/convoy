package organisations

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/users"
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

	db := postgres.NewFromConnection(conn)

	return db, ctx
}

func seedUser(t *testing.T, db database.Database) *datastore.User {
	t.Helper()

	userRepo := users.New(log.NewLogger(io.Discard), db)
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

func seedOrganisation(t *testing.T, db database.Database, customDomain, assignedDomain string) *datastore.Organisation {
	t.Helper()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Organisation",
		OwnerID: user.UID,
	}

	if customDomain != "" {
		org.CustomDomain.String = customDomain
		org.CustomDomain.Valid = true
	}

	if assignedDomain != "" {
		org.AssignedDomain.String = assignedDomain
		org.AssignedDomain.Valid = true
	}

	service := New(log.NewLogger(os.Stdout), db)
	err := service.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func TestCreateOrganisation_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	service := New(log.NewLogger(os.Stdout), db)

	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Organisation",
		OwnerID: user.UID,
	}

	err := service.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify organisation was created
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, org.Name, fetched.Name)
	require.Equal(t, org.OwnerID, fetched.OwnerID)
}

func TestCreateOrganisation_WithCustomDomain(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	service := New(log.NewLogger(os.Stdout), db)

	org := &datastore.Organisation{
		UID:          ulid.Make().String(),
		Name:         "Test Organisation",
		OwnerID:      user.UID,
		CustomDomain: null.StringFrom("custom.example.com"),
	}

	err := service.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify organisation was created with custom domain
	fetched, err := service.FetchOrganisationByCustomDomain(ctx, "custom.example.com")
	require.NoError(t, err)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, "custom.example.com", fetched.CustomDomain.String)
	require.True(t, fetched.CustomDomain.Valid)
}

func TestCreateOrganisation_WithAssignedDomain(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	service := New(log.NewLogger(os.Stdout), db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "Test Organisation",
		OwnerID:        user.UID,
		AssignedDomain: null.StringFrom("assigned.convoy.io"),
	}

	err := service.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify organisation was created with assigned domain
	fetched, err := service.FetchOrganisationByAssignedDomain(ctx, "assigned.convoy.io")
	require.NoError(t, err)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, "assigned.convoy.io", fetched.AssignedDomain.String)
	require.True(t, fetched.AssignedDomain.Valid)
}

func TestCreateOrganisation_WithBothDomains(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	service := New(log.NewLogger(os.Stdout), db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "Test Organisation",
		OwnerID:        user.UID,
		CustomDomain:   null.StringFrom("custom.example.com"),
		AssignedDomain: null.StringFrom("assigned.convoy.io"),
	}

	err := service.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify both domains are set
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, "custom.example.com", fetched.CustomDomain.String)
	require.Equal(t, "assigned.convoy.io", fetched.AssignedDomain.String)
}

func TestCreateOrganisation_NilOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.CreateOrganisation(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation cannot be nil")
}

func TestCreateOrganisation_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create a user first (organisations require a valid owner_id)
	user := seedUser(t, db)

	service := New(log.NewLogger(os.Stdout), db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "Persistence Test Org",
		OwnerID:        user.UID,
		CustomDomain:   null.StringFrom("persist.test.com"),
		AssignedDomain: null.StringFrom("persist.convoy.io"),
	}

	// Create organisation
	err := service.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create a new service instance to ensure no caching
	newService := New(log.NewLogger(os.Stdout), db)

	// Fetch and verify all fields match
	fetched, err := newService.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, org.Name, fetched.Name)
	require.Equal(t, org.OwnerID, fetched.OwnerID)
	require.Equal(t, org.CustomDomain.String, fetched.CustomDomain.String)
	require.Equal(t, org.AssignedDomain.String, fetched.AssignedDomain.String)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)
}
