package delivery_attempts

import (
	"context"
	"os"
	"testing"
	"time"

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

func TestCreateDeliveryAttempt_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(nil), db)

	attempt := &datastore.DeliveryAttempt{
		UID:              ulid.Make().String(),
		URL:              "https://example.com/webhook",
		Method:           "POST",
		APIVersion:       "2023.12.25",
		EndpointID:       endpoint.UID,
		EventDeliveryId:  eventDelivery.UID,
		ProjectId:        project.UID,
		IPAddress:        "192.168.1.1",
		HttpResponseCode: "200",
		Status:           true,
	}

	err := service.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	// Verify the attempt was created
	fetched, err := service.FindDeliveryAttemptById(ctx, eventDelivery.UID, attempt.UID)
	require.NoError(t, err)
	require.Equal(t, attempt.UID, fetched.UID)
	require.Equal(t, attempt.URL, fetched.URL)
	require.Equal(t, attempt.Method, fetched.Method)
	require.Equal(t, attempt.Status, fetched.Status)
}

func TestCreateDeliveryAttempt_WithHeaders(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(nil), db)

	requestHeaders := datastore.HttpHeader{
		"Content-Type": "application/json",
		"X-Custom":     "test-value",
	}
	responseHeaders := datastore.HttpHeader{
		"Content-Type": "application/json",
		"X-Response":   "response-value",
	}

	attempt := &datastore.DeliveryAttempt{
		UID:              ulid.Make().String(),
		URL:              "https://example.com/webhook",
		Method:           "POST",
		APIVersion:       "2023.12.25",
		EndpointID:       endpoint.UID,
		EventDeliveryId:  eventDelivery.UID,
		ProjectId:        project.UID,
		RequestHeader:    requestHeaders,
		ResponseHeader:   responseHeaders,
		HttpResponseCode: "201",
		ResponseData:     []byte(`{"message": "success"}`),
		Status:           true,
	}

	err := service.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	// Verify headers were stored
	fetched, err := service.FindDeliveryAttemptById(ctx, eventDelivery.UID, attempt.UID)
	require.NoError(t, err)
	require.Equal(t, requestHeaders, fetched.RequestHeader)
	require.Equal(t, responseHeaders, fetched.ResponseHeader)
	require.Equal(t, string(attempt.ResponseData), string(fetched.ResponseData))
}

func TestCreateDeliveryAttempt_FailedAttempt(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(nil), db)

	attempt := &datastore.DeliveryAttempt{
		UID:              ulid.Make().String(),
		URL:              "https://example.com/webhook",
		Method:           "POST",
		APIVersion:       "2023.12.25",
		EndpointID:       endpoint.UID,
		EventDeliveryId:  eventDelivery.UID,
		ProjectId:        project.UID,
		HttpResponseCode: "500",
		Error:            "connection timeout",
		Status:           false,
	}

	err := service.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	// Verify the failed attempt was created
	fetched, err := service.FindDeliveryAttemptById(ctx, eventDelivery.UID, attempt.UID)
	require.NoError(t, err)
	require.Equal(t, attempt.Error, fetched.Error)
	require.False(t, fetched.Status)
}

func TestFindDeliveryAttempts_MultipleAttempts(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(nil), db)

	// Create multiple attempts
	for i := 0; i < 5; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          i%2 == 0, // Alternate between success and failure
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Fetch all attempts
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 5)
}

func TestFindDeliveryAttemptById_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(nil), db)

	_, err := service.FindDeliveryAttemptById(ctx, eventDelivery.UID, "nonexistent-id")
	require.Error(t, err)
	require.Equal(t, datastore.ErrDeliveryAttemptNotFound, err)
}

// Helper functions for test setup
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

func seedTestData(t *testing.T, db database.Database, ctx context.Context) *datastore.Project {
	t.Helper()

	// Create user with unique email
	userRepo := postgres.NewUserRepo(db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		Email:     "test-" + ulid.Make().String() + "@example.com", // Unique email per test
		FirstName: "Test",
		LastName:  "User",
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

func seedEndpoint(t *testing.T, db database.Database, ctx context.Context, project *datastore.Project) *datastore.Endpoint {
	t.Helper()

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

	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}

func seedEventDelivery(t *testing.T, db database.Database, ctx context.Context, project *datastore.Project, endpoint *datastore.Endpoint) *datastore.EventDelivery {
	t.Helper()

	// First create an event
	eventRepo := postgres.NewEventRepo(db)
	event := &datastore.Event{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: datastore.EventType("test.event"),
		Data:      []byte(`{"test": "data"}`),
	}
	err := eventRepo.CreateEvent(ctx, event)
	require.NoError(t, err)

	// Create a subscription
	subscriptionRepo := subscriptions.New(log.NewLogger(os.Stdout), db)
	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "Test Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
		RateLimitConfig: &datastore.DefaultRateLimitConfig,
		RetryConfig:     &datastore.DefaultRetryConfig,
		AlertConfig:     &datastore.DefaultAlertConfig,
	}
	err = subscriptionRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err)

	// Now create event delivery with valid event_id and subscription_id
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)

	eventDelivery := &datastore.EventDelivery{
		UID:            ulid.Make().String(),
		ProjectID:      project.UID,
		EventID:        event.UID,
		EndpointID:     endpoint.UID,
		Status:         datastore.ScheduledEventStatus,
		SubscriptionID: subscription.UID,
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"key": "value"}`),
			Raw:             `{"key": "value"}`,
			Strategy:        datastore.LinearStrategyProvider,
			NextSendTime:    time.Now(),
			NumTrials:       0,
			IntervalSeconds: 0,
			RetryLimit:      3,
		},
	}

	err = eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
	require.NoError(t, err)

	return eventDelivery
}
