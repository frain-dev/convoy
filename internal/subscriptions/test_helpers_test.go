package subscriptions

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
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

// Test environment
var testEnv *testenv.Environment

// TestMain initializes the test environment
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

// setupTestDB initializes a test database connection with encryption key manager
func setupTestDB(t *testing.T) (database.Database, context.Context, *Service) {
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

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	return db, ctx, service
}

// seedUser creates a test user
func seedUser(t *testing.T, db database.Database) *datastore.User {
	t.Helper()

	uid := ulid.Make().String()
	user := &datastore.User{
		UID:       uid,
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@user.com", uid),
	}

	userRepo := users.New(log.NewLogger(io.Discard), db)
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}

// seedOrganisation creates a test organisation
func seedOrganisation(t *testing.T, db database.Database, user *datastore.User) *datastore.Organisation {
	t.Helper()

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "Test Org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("", false),
		AssignedDomain: null.NewString("test.convoy.io", true),
	}

	logger := log.NewLogger(os.Stdout)
	orgRepo := organisations.New(logger, db)
	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

// seedProject creates a test project
func seedProject(t *testing.T, db database.Database, org *datastore.Organisation) *datastore.Project {
	t.Helper()

	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		OrganisationID: org.UID,
		Type:           datastore.OutgoingProject,
		Config:         &projectConfig,
	}

	logger := log.NewLogger(os.Stdout)
	projectRepo := projects.New(logger, db)
	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	return project
}

// seedEndpoint creates a test endpoint
func seedEndpoint(t *testing.T, db database.Database, project *datastore.Project) *datastore.Endpoint {
	t.Helper()

	endpoint := &datastore.Endpoint{
		UID:         ulid.Make().String(),
		ProjectID:   project.UID,
		Name:        "Test Endpoint",
		Url:         "https://api.example.com/webhook",
		Status:      datastore.ActiveEndpointStatus,
		Secrets:     []datastore.Secret{{UID: ulid.Make().String(), Value: "secret"}},
		HttpTimeout: 30,
	}

	endpointRepo := postgres.NewEndpointRepo(db)
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}

// seedSource creates a test source
func seedSource(t *testing.T, db database.Database, project *datastore.Project) *datastore.Source {
	t.Helper()

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Test Source",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Verifier: &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Header:   "X-Webhook-Signature",
				Hash:     "SHA256",
				Secret:   "test-secret",
				Encoding: datastore.HexEncoding,
			},
		},
	}

	logger := log.NewLogger(os.Stdout)
	sourceRepo := sources.New(logger, db)
	err := sourceRepo.CreateSource(context.Background(), source)
	require.NoError(t, err)

	return source
}

// seedDevice creates a test device
func seedDevice(t *testing.T, db database.Database, project *datastore.Project) *datastore.Device {
	t.Helper()

	device := &datastore.Device{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		HostName:  "test-device.example.com",
		Status:    datastore.DeviceStatusOnline,
	}

	// Create device directly via SQL since there's no dedicated repository
	// Note: endpoint_id was removed from devices table in migration 1679836136.sql
	query := `INSERT INTO convoy.devices (id, project_id, host_name, status, last_seen_at, created_at, updated_at)
              VALUES ($1, $2, $3, $4, NOW(), NOW(), NOW())`

	_, err := db.GetConn().Exec(context.Background(), query,
		device.UID, device.ProjectID, device.HostName, device.Status)
	require.NoError(t, err)

	return device
}

// seedTestData creates all test fixtures
func seedTestData(t *testing.T, db database.Database) (*datastore.Project, *datastore.Endpoint, *datastore.Source, *datastore.Device) {
	t.Helper()

	user := seedUser(t, db)
	org := seedOrganisation(t, db, user)
	project := seedProject(t, db, org)
	endpoint := seedEndpoint(t, db, project)
	source := seedSource(t, db, project)
	device := seedDevice(t, db, project)

	return project, endpoint, source, device
}

// createTestSubscription creates a test subscription with default configurations
func createTestSubscription(project *datastore.Project, endpoint *datastore.Endpoint, source *datastore.Source) *datastore.Subscription {
	return &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "Test Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		SourceID:   source.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"user.created", "user.updated"},
			Filter: datastore.FilterSchema{
				Headers:     datastore.M{"X-Company-ID": "123"},
				Body:        datastore.M{"event": "user"},
				RawHeaders:  datastore.M{"X-Company-ID": "123"},
				RawBody:     datastore.M{"event": "user"},
				IsFlattened: true,
			},
		},
		AlertConfig: &datastore.AlertConfiguration{
			Count:     10,
			Threshold: "1h",
		},
		RetryConfig: &datastore.RetryConfiguration{
			Type:       datastore.LinearStrategyProvider,
			Duration:   60,
			RetryCount: 3,
		},
		RateLimitConfig: &datastore.RateLimitConfiguration{
			Count:    100,
			Duration: 60,
		},
		DeliveryMode: datastore.AtLeastOnceDeliveryMode,
	}
}

// createTestCLISubscription creates a CLI subscription for testing
func createTestCLISubscription(project *datastore.Project, device *datastore.Device) *datastore.Subscription {
	return &datastore.Subscription{
		UID:       ulid.Make().String(),
		Name:      "Test CLI Subscription",
		Type:      datastore.SubscriptionTypeCLI,
		ProjectID: project.UID,
		DeviceID:  device.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers:     datastore.M{},
				Body:        datastore.M{},
				RawHeaders:  datastore.M{},
				RawBody:     datastore.M{},
				IsFlattened: true,
			},
		},
		DeliveryMode: datastore.AtMostOnceDeliveryMode,
	}
}

// assertSubscriptionEqual verifies two subscriptions are equal
func assertSubscriptionEqual(t *testing.T, expected, actual *datastore.Subscription) {
	t.Helper()

	require.Equal(t, expected.UID, actual.UID)
	require.Equal(t, expected.Name, actual.Name)
	require.Equal(t, expected.Type, actual.Type)
	require.Equal(t, expected.ProjectID, actual.ProjectID)
	require.Equal(t, expected.EndpointID, actual.EndpointID)
	require.Equal(t, expected.SourceID, actual.SourceID)
	require.Equal(t, expected.DeviceID, actual.DeviceID)
	require.Equal(t, expected.DeliveryMode, actual.DeliveryMode)

	// Assert configs
	if expected.FilterConfig != nil {
		require.NotNil(t, actual.FilterConfig)
		require.ElementsMatch(t, expected.FilterConfig.EventTypes, actual.FilterConfig.EventTypes)
	}

	if expected.AlertConfig != nil {
		require.NotNil(t, actual.AlertConfig)
		require.Equal(t, expected.AlertConfig.Count, actual.AlertConfig.Count)
		require.Equal(t, expected.AlertConfig.Threshold, actual.AlertConfig.Threshold)
	}

	if expected.RetryConfig != nil {
		require.NotNil(t, actual.RetryConfig)
		require.Equal(t, expected.RetryConfig.Type, actual.RetryConfig.Type)
		require.Equal(t, expected.RetryConfig.Duration, actual.RetryConfig.Duration)
		require.Equal(t, expected.RetryConfig.RetryCount, actual.RetryConfig.RetryCount)
	}

	if expected.RateLimitConfig != nil {
		require.NotNil(t, actual.RateLimitConfig)
		require.Equal(t, expected.RateLimitConfig.Count, actual.RateLimitConfig.Count)
		require.Equal(t, expected.RateLimitConfig.Duration, actual.RateLimitConfig.Duration)
	}
}
