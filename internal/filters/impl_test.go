package filters

import (
	"context"
	"os"
	"strings"
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
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	testEnv *testenv.Environment
)

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
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data any, changelog any) {})

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

func seedTestData(t *testing.T, db database.Database) (*datastore.Project, *datastore.Subscription) {
	t.Helper()
	logger := log.NewLogger(os.Stdout)

	ctx := context.Background()

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

	// Create endpoint
	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Test Endpoint",
		Status:    datastore.ActiveEndpointStatus,
		AppID:     ulid.Make().String(),
		Url:       "https://example.com/webhook",
		Secrets: []datastore.Secret{
			{Value: "test-secret"},
		},
	}
	err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create subscription
	subRepo := subscriptions.New(log.NewLogger(os.Stdout), db)
	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		Name:       "Test Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
		AlertConfig: &datastore.AlertConfiguration{
			Count:     10,
			Threshold: "1m",
		},
		RetryConfig: &datastore.RetryConfiguration{
			Type:       "linear",
			Duration:   3,
			RetryCount: 10,
		},
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
		RateLimitConfig: &datastore.RateLimitConfiguration{
			Count:    100,
			Duration: 60,
		},
	}
	err = subRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err)

	// Clean up any auto-created filters from subscription creation
	// Tests will create their own filters as needed
	filterRepo := New(log.NewLogger(os.Stdout), db)
	existingFilters, err := filterRepo.FindFiltersBySubscriptionID(ctx, subscription.UID)
	require.NoError(t, err)
	for _, filter := range existingFilters {
		err = filterRepo.DeleteFilter(ctx, filter.UID)
		require.NoError(t, err)
	}

	return project, subscription
}

func seedEventType(t *testing.T, db database.Database, projectID, eventType string) {
	t.Helper()

	ctx := context.Background()
	eventTypeRepo := postgres.NewEventTypesRepo(db)

	et := &datastore.ProjectEventType{
		UID:       ulid.Make().String(),
		ProjectId: projectID,
		Name:      eventType,
	}
	err := eventTypeRepo.CreateEventType(ctx, et)
	// Ignore duplicate key errors - event type may already exist from subscription creation
	if err != nil && !strings.Contains(err.Error(), "duplicate key") && !strings.Contains(err.Error(), "SQLSTATE 23505") {
		require.NoError(t, err)
	}
}

func createFilterService(t *testing.T, db database.Database) *Service {
	t.Helper()
	logger := log.NewLogger(os.Stdout)
	return New(logger, db)
}
