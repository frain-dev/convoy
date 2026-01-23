package projects

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
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	testEnv    *testenv.Environment
	testLogger log.StdLogger
)

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	testEnv = res
	testLogger = log.NewLogger(os.Stdout)

	code := m.Run()

	if err = cleanup(); err != nil {
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
	dbHooks.RegisterHook(datastore.ProjectUpdated, func(ctx context.Context, data interface{}, changelog interface{}) {})

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

	user := seedUser(t, db)

	orgService := organisations.New(testLogger, db)
	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("Test Org %s", ulid.Make().String()),
		OwnerID:        user.UID,
		CustomDomain:   null.String{},
		AssignedDomain: null.String{},
	}

	err := orgService.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func getDefaultProjectConfig() *datastore.ProjectConfig {
	return &datastore.ProjectConfig{
		SearchPolicy:                  "",
		MaxIngestSize:                 5242880, // 5MB
		MultipleEndpointSubscriptions: false,
		ReplayAttacks:                 false,
		DisableEndpoint:               false,
		RateLimit: &datastore.RateLimitConfiguration{
			Count:    5000,
			Duration: 60,
		},
		Strategy: &datastore.StrategyConfiguration{
			Type:       datastore.LinearStrategyProvider,
			Duration:   10,
			RetryCount: 3,
		},
		Signature: &datastore.SignatureConfiguration{
			Header:   config.DefaultSignatureHeader,
			Versions: datastore.SignatureVersions{},
		},
		SSL: &datastore.SSLConfiguration{
			EnforceSecureEndpoints: false,
		},
		MetaEvent: &datastore.MetaEventConfiguration{
			IsEnabled: false,
			Type:      "",
			EventType: []string{},
			URL:       "",
			Secret:    "",
			PubSub:    &datastore.PubSubConfig{},
		},
		CircuitBreaker: &datastore.CircuitBreakerConfiguration{
			SampleRate:                  100,
			ErrorTimeout:                30,
			FailureThreshold:            50,
			SuccessThreshold:            10,
			ObservabilityWindow:         5,
			MinimumRequestCount:         10,
			ConsecutiveFailureThreshold: 5,
		},
	}
}

func seedProject(t *testing.T, db database.Database, org *datastore.Organisation) *datastore.Project {
	t.Helper()

	service := New(testLogger, db)
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("Test Project %s", ulid.Make().String()),
		Type:           datastore.OutgoingProject,
		LogoURL:        "",
		OrganisationID: org.UID,
		RetainedEvents: 0,
		Config:         getDefaultProjectConfig(),
	}

	err := service.CreateProject(context.Background(), project)
	require.NoError(t, err)

	// Fetch back to get full details including ProjectConfigID
	fetched, err := service.FetchProjectByID(context.Background(), project.UID)
	require.NoError(t, err)

	return fetched
}

func seedProjectWithCustomConfig(t *testing.T, db database.Database, org *datastore.Organisation) *datastore.Project {
	t.Helper()

	service := New(testLogger, db)
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("Custom Project %s", ulid.Make().String()),
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config: &datastore.ProjectConfig{
			SearchPolicy:  "custom-policy",
			MaxIngestSize: 10485760, // 10MB
			ReplayAttacks: true,
			RateLimit: &datastore.RateLimitConfiguration{
				Count:    10000,
				Duration: 120,
			},
			Strategy: &datastore.StrategyConfiguration{
				Type:       datastore.ExponentialStrategyProvider,
				Duration:   20,
				RetryCount: 5,
			},
			Signature: &datastore.SignatureConfiguration{
				Header:   config.DefaultSignatureHeader,
				Versions: datastore.SignatureVersions{},
			},
			SSL: &datastore.SSLConfiguration{
				EnforceSecureEndpoints: true,
			},
			MetaEvent: &datastore.MetaEventConfiguration{
				IsEnabled: true,
				Type:      datastore.HTTPMetaEvent,
				URL:       "https://example.com/meta",
				Secret:    "secret123",
			},
			CircuitBreaker: &datastore.CircuitBreakerConfiguration{
				SampleRate:                  200,
				ErrorTimeout:                60,
				FailureThreshold:            75,
				SuccessThreshold:            20,
				ObservabilityWindow:         10,
				MinimumRequestCount:         20,
				ConsecutiveFailureThreshold: 10,
			},
		},
	}

	err := service.CreateProject(context.Background(), project)
	require.NoError(t, err)

	fetched, err := service.FetchProjectByID(context.Background(), project.UID)
	require.NoError(t, err)

	return fetched
}

func seedEndpoint(t *testing.T, db database.Database, project *datastore.Project, status datastore.EndpointStatus) *datastore.Endpoint {
	t.Helper()

	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:               ulid.Make().String(),
		ProjectID:         project.UID,
		Name:              fmt.Sprintf("Test Endpoint %s", ulid.Make().String()),
		Url:               "https://example.com/webhook",
		Status:            status,
		Description:       "Test endpoint",
		HttpTimeout:       30,
		RateLimit:         0,
		RateLimitDuration: 0,
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String(), Value: "secret123"},
		},
	}

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}

func seedEvent(t *testing.T, db database.Database, project *datastore.Project, endpoint *datastore.Endpoint) *datastore.Event {
	t.Helper()

	eventRepo := postgres.NewEventRepo(db)

	var endpoints []string
	if endpoint != nil {
		endpoints = []string{endpoint.UID}
	}

	event := &datastore.Event{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Endpoints: endpoints,
		EventType: datastore.EventType("test.event"),
		Data:      []byte(`{"test": "data"}`),
	}

	err := eventRepo.CreateEvent(context.Background(), event)
	require.NoError(t, err)

	return event
}

func seedSubscription(t *testing.T, db database.Database, project *datastore.Project, endpoint *datastore.Endpoint) *datastore.Subscription {
	t.Helper()

	subRepo := postgres.NewSubscriptionRepo(db)
	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		Name:       fmt.Sprintf("Test Subscription %s", ulid.Make().String()),
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
	}

	err := subRepo.CreateSubscription(context.Background(), project.UID, subscription)
	require.NoError(t, err)

	return subscription
}

func seedSource(t *testing.T, db database.Database, project *datastore.Project) {
	t.Helper()

	sourceRepo := sources.New(log.NewLogger(io.Discard), db)
	source := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      fmt.Sprintf("Test Source %s", ulid.Make().String()),
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Verifier:  &datastore.VerifierConfig{},
	}

	err := sourceRepo.CreateSource(context.Background(), source)
	require.NoError(t, err)
}
