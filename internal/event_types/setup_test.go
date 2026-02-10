package event_types

import (
	"context"
	"fmt"
	"os"
	"testing"

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
	"github.com/frain-dev/convoy/internal/users"
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

func createEventTypeService(t *testing.T, db database.Database) *Service {
	t.Helper()
	return New(log.NewLogger(os.Stdout), db)
}

func seedTestData(t *testing.T, db database.Database) (*datastore.User, *datastore.Organisation, *datastore.Project) {
	ctx := context.Background()
	logger := log.NewLogger(os.Stdout)

	// Create user
	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
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
	projectRepo := projects.New(logger, db)
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

func seedEventType(t *testing.T, db database.Database, project *datastore.Project, name, description, category string, jsonSchema []byte) *datastore.ProjectEventType {
	ctx := context.Background()
	service := createEventTypeService(t, db)

	eventType := &datastore.ProjectEventType{
		UID:         ulid.Make().String(),
		Name:        name,
		ProjectId:   project.UID,
		Description: description,
		Category:    category,
		JSONSchema:  jsonSchema,
	}

	err := service.CreateEventType(ctx, eventType)
	require.NoError(t, err)

	return eventType
}
