package organisation_members

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
	fmt.Println("TestMain started")
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Printf("testenv.Launch failed: %v\n", err)
		panic(err)
	}
	fmt.Println("testenv.Launch succeeded")
	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("failed to cleanup: %v\n", err)
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

func createOrgMemberService(t *testing.T, db database.Database) *Service {
	t.Helper()

	logger := log.NewLogger(os.Stdout)
	return New(logger, db)
}

func seedUser(t *testing.T, db database.Database, email string) *datastore.User {
	t.Helper()

	if email == "" {
		email = fmt.Sprintf("test-%s@example.com", ulid.Make().String())
	}

	userRepo := postgres.NewUserRepo(db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}

func seedOrganisation(t *testing.T, db database.Database, ownerID string) *datastore.Organisation {
	t.Helper()

	logger := log.NewLogger(os.Stdout)
	orgRepo := organisations.New(logger, db)

	org := &datastore.Organisation{
		UID:       ulid.Make().String(),
		Name:      fmt.Sprintf("Test Org %s", ulid.Make().String()),
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func seedOrganisationMember(t *testing.T, db database.Database, orgID, userID string, role auth.Role) *datastore.OrganisationMember {
	t.Helper()

	service := createOrgMemberService(t, db)

	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: orgID,
		UserID:         userID,
		Role:           role,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := service.CreateOrganisationMember(context.Background(), member)
	require.NoError(t, err)

	return member
}

func seedProject(t *testing.T, db database.Database, orgID string) *datastore.Project {
	t.Helper()

	projectRepo := projects.New(log.NewLogger(os.Stdout), db)

	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("Test Project %s", ulid.Make().String()),
		Type:           datastore.OutgoingProject,
		OrganisationID: orgID,
		Config:         &projectConfig,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	return project
}

func seedEndpoint(t *testing.T, db database.Database, projectID string) *datastore.Endpoint {
	t.Helper()

	endpointRepo := postgres.NewEndpointRepo(db)

	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: projectID,
		Name:      fmt.Sprintf("Test Endpoint %s", ulid.Make().String()),
		Url:       "https://example.com/webhook",
		Secrets:   make([]datastore.Secret, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, projectID)
	require.NoError(t, err)

	return endpoint
}
