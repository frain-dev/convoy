package event_deliveries

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/pkg/httpheader"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Printf("Failed to launch test environment: %v\n", err)
		os.Exit(1)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("Failed to cleanup test infrastructure: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (*Service, database.Database) {
	t.Helper()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data any, changelog any) {})
	dbHooks.RegisterHook(datastore.EventDeliveryUpdated, func(ctx context.Context, data any, changelog any) {})

	db := postgres.NewFromConnection(conn)

	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	logger := log.New("convoy", log.LevelInfo)
	return New(logger, db), db
}

func seedTestProject(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()

	logger := log.New("convoy", log.LevelInfo)
	ctx := context.Background()

	userRepo := users.New(logger, db)
	userID := ulid.Make().String()
	user := &datastore.User{
		UID:       userID,
		Email:     fmt.Sprintf("test-%s@example.com", userID),
		FirstName: "Test",
		LastName:  "User",
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	projectRepo := projects.New(logger, db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err = projectRepo.CreateProject(ctx, project)
	require.NoError(t, err)

	return project
}

func seedTestEndpoint(t *testing.T, db database.Database, projectID string) *datastore.Endpoint {
	t.Helper()

	ctx := context.Background()
	endpointRepo := endpoints.New(log.New("convoy", log.LevelInfo), db)

	endpointID := ulid.Make().String()
	endpoint := &datastore.Endpoint{
		UID:          endpointID,
		ProjectID:    projectID,
		Name:         fmt.Sprintf("Test Endpoint %s", endpointID),
		Url:          fmt.Sprintf("https://example.com/webhook/%s", endpointID),
		Status:       datastore.ActiveEndpointStatus,
		SupportEmail: fmt.Sprintf("test-%s@example.com", endpointID),
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String(), Value: "test-secret-value"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := endpointRepo.CreateEndpoint(ctx, endpoint, projectID)
	require.NoError(t, err)

	return endpoint
}

func seedTestSource(t *testing.T, db database.Database, projectID string) *datastore.Source {
	t.Helper()

	logger := log.New("convoy", log.LevelInfo)
	ctx := context.Background()
	sourceRepo := sources.New(logger, db)

	sourceID := ulid.Make().String()
	source := &datastore.Source{
		UID:       sourceID,
		ProjectID: projectID,
		Name:      fmt.Sprintf("Test Source %s", sourceID),
		MaskID:    fmt.Sprintf("src_%s", sourceID),
		Type:      datastore.HTTPSource,
		Verifier:  &datastore.VerifierConfig{Type: datastore.NoopVerifier},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err)

	return source
}

func seedDevice(t *testing.T, db database.Database, projectID string) *datastore.Device {
	t.Helper()

	device := &datastore.Device{
		UID:       ulid.Make().String(),
		ProjectID: projectID,
		HostName:  "test-device.example.com",
		Status:    datastore.DeviceStatusOnline,
	}

	query := `INSERT INTO convoy.devices (id, project_id, host_name, status, last_seen_at, created_at, updated_at)
              VALUES ($1, $2, $3, $4, NOW(), NOW(), NOW())`
	_, err := db.GetConn().Exec(context.Background(), query,
		device.UID, device.ProjectID, device.HostName, device.Status)
	require.NoError(t, err)

	return device
}

func seedSubscription(t *testing.T, db database.Database, projectID, endpointID, sourceID string) *datastore.Subscription {
	t.Helper()

	logger := log.New("convoy", log.LevelInfo)
	ctx := context.Background()
	subRepo := subscriptions.New(logger, db)

	sub := &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "Test Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  projectID,
		EndpointID: endpointID,
		SourceID:   sourceID,
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

	err := subRepo.CreateSubscription(ctx, projectID, sub)
	require.NoError(t, err)

	return sub
}

func seedEvent(t *testing.T, db database.Database, projectID, endpointID, sourceID string) *datastore.Event {
	t.Helper()

	logger := log.New("convoy", log.LevelInfo)
	ctx := context.Background()
	eventRepo := events.New(logger, db)

	eventID := ulid.Make().String()
	now := time.Now()
	event := &datastore.Event{
		UID:            eventID,
		EventType:      datastore.EventType("test.event"),
		ProjectID:      projectID,
		SourceID:       sourceID,
		Endpoints:      []string{endpointID},
		Headers:        httpheader.HTTPHeader{"X-Test": []string{"value"}},
		Raw:            `{"test": "data"}`,
		Data:           json.RawMessage(`{"test": "data"}`),
		IdempotencyKey: fmt.Sprintf("idempotency-%s", eventID),
		CreatedAt:      now,
		UpdatedAt:      now,
		AcknowledgedAt: null.TimeFrom(now),
	}
	err := eventRepo.CreateEvent(ctx, event)
	require.NoError(t, err)

	return event
}

func createTestEventDelivery(t *testing.T, projectID, eventID, endpointID, subscriptionID string) *datastore.EventDelivery {
	t.Helper()

	return &datastore.EventDelivery{
		UID:            ulid.Make().String(),
		ProjectID:      projectID,
		EventID:        eventID,
		EndpointID:     endpointID,
		SubscriptionID: subscriptionID,
		Headers:        httpheader.HTTPHeader{"X-Delivery": []string{"test"}},
		Status:         datastore.ScheduledEventStatus,
		Metadata: &datastore.Metadata{
			NumTrials:       0,
			RetryLimit:      3,
			IntervalSeconds: 60,
		},
		Description:    "Test delivery",
		URLQueryParams: "key=value",
		IdempotencyKey: fmt.Sprintf("idempotency-%s", ulid.Make().String()),
		EventType:      datastore.EventType("test.event"),
		AcknowledgedAt: null.TimeFrom(time.Now()),
	}
}

func defaultSearchParams() datastore.SearchParams {
	return datastore.SearchParams{
		CreatedAtStart: time.Now().Add(-24 * time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
	}
}

func TestCreateEventDelivery(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)

		err := service.CreateEventDelivery(ctx, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, delivery.UID, found.UID)
		require.Equal(t, delivery.ProjectID, found.ProjectID)
		require.Equal(t, delivery.EventID, found.EventID)
		require.Equal(t, delivery.EndpointID, found.EndpointID)
		require.Equal(t, delivery.SubscriptionID, found.SubscriptionID)
		require.Equal(t, delivery.Status, found.Status)
		require.Equal(t, delivery.Description, found.Description)
	})

	t.Run("WithCLIMetadata", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		delivery.CLIMetadata = &datastore.CLIMetadata{
			EventType: string(delivery.EventType),
			SourceID:  source.UID,
		}

		err := service.CreateEventDelivery(ctx, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.NotNil(t, found.CLIMetadata)
		require.Equal(t, delivery.CLIMetadata.EventType, found.CLIMetadata.EventType)
		require.Equal(t, delivery.CLIMetadata.SourceID, found.CLIMetadata.SourceID)
	})

	t.Run("DefaultDeliveryMode", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		delivery.DeliveryMode = "" // should default to AtLeastOnce

		err := service.CreateEventDelivery(ctx, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, datastore.AtLeastOnceDeliveryMode, found.DeliveryMode)
	})
}

func TestCreateEventDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("BatchCreate", func(t *testing.T) {
		deliveries := make([]*datastore.EventDelivery, 5)
		for i := 0; i < 5; i++ {
			deliveries[i] = createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		}

		err := service.CreateEventDeliveries(ctx, deliveries)
		require.NoError(t, err)

		for _, d := range deliveries {
			found, err := service.FindEventDeliveryByID(ctx, project.UID, d.UID)
			require.NoError(t, err)
			require.Equal(t, d.UID, found.UID)
		}
	})

	t.Run("EmptySlice", func(t *testing.T) {
		err := service.CreateEventDeliveries(ctx, []*datastore.EventDelivery{})
		require.NoError(t, err)
	})
}

func TestFindEventDeliveryByID(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		err := service.CreateEventDelivery(ctx, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, delivery.UID, found.UID)
		require.Equal(t, delivery.ProjectID, found.ProjectID)
		require.Equal(t, delivery.EventID, found.EventID)
		require.Equal(t, delivery.EndpointID, found.EndpointID)
		require.Equal(t, delivery.SubscriptionID, found.SubscriptionID)
		require.Equal(t, delivery.Status, found.Status)

		// Verify JOINed metadata is populated
		require.NotNil(t, found.Endpoint)
		require.Equal(t, endpoint.UID, found.Endpoint.UID)
		require.Equal(t, endpoint.Name, found.Endpoint.Name)

		require.NotNil(t, found.Event)

		require.NotNil(t, found.Source)
		require.Equal(t, source.UID, found.Source.UID)
		require.Equal(t, source.Name, found.Source.Name)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := service.FindEventDeliveryByID(ctx, project.UID, ulid.Make().String())
		require.Error(t, err)
		require.Equal(t, datastore.ErrEventDeliveryNotFound, err)
	})
}

func TestFindEventDeliveryByIDSlim(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		err := service.CreateEventDelivery(ctx, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByIDSlim(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, delivery.UID, found.UID)
		require.Equal(t, delivery.ProjectID, found.ProjectID)
		require.Equal(t, delivery.EventID, found.EventID)

		// Slim should NOT have JOINed metadata
		require.Nil(t, found.Endpoint)
		require.Nil(t, found.Event)
		require.Nil(t, found.Source)
		require.Nil(t, found.Device)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := service.FindEventDeliveryByIDSlim(ctx, project.UID, ulid.Make().String())
		require.Error(t, err)
		require.Equal(t, datastore.ErrEventDeliveryNotFound, err)
	})
}

func TestFindEventDeliveriesByIDs(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Multiple", func(t *testing.T) {
		ids := make([]string, 3)
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			ids[i] = d.UID
		}

		results, err := service.FindEventDeliveriesByIDs(ctx, project.UID, ids)
		require.NoError(t, err)
		require.Len(t, results, 3)
	})

	t.Run("Empty", func(t *testing.T) {
		results, err := service.FindEventDeliveriesByIDs(ctx, project.UID, []string{})
		require.NoError(t, err)
		require.Empty(t, results)
	})
}

func TestFindEventDeliveriesByEventID(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		results, err := service.FindEventDeliveriesByEventID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 3)

		for _, d := range results {
			require.Equal(t, event.UID, d.EventID)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		results, err := service.FindEventDeliveriesByEventID(ctx, project.UID, ulid.Make().String())
		require.NoError(t, err)
		require.Empty(t, results)
	})
}

func TestCountDeliveriesByStatus(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("WithEvents", func(t *testing.T) {
		// Create 3 Scheduled deliveries
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.ScheduledEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		// Create 2 Success deliveries
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.SuccessEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		count, err := service.CountDeliveriesByStatus(ctx, project.UID, datastore.ScheduledEventStatus, defaultSearchParams())
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(3))

		count, err = service.CountDeliveriesByStatus(ctx, project.UID, datastore.SuccessEventStatus, defaultSearchParams())
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(2))
	})
}

func TestUpdateStatusOfEventDelivery(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, delivery))

		err := service.UpdateStatusOfEventDelivery(ctx, project.UID, *delivery, datastore.SuccessEventStatus)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, datastore.SuccessEventStatus, found.Status)
	})
}

func TestUpdateStatusOfEventDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("BulkUpdate", func(t *testing.T) {
		ids := make([]string, 3)
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			ids[i] = d.UID
		}

		err := service.UpdateStatusOfEventDeliveries(ctx, project.UID, ids, datastore.FailureEventStatus)
		require.NoError(t, err)

		for _, id := range ids {
			found, err := service.FindEventDeliveryByID(ctx, project.UID, id)
			require.NoError(t, err)
			require.Equal(t, datastore.FailureEventStatus, found.Status)
		}
	})
}

func TestFindDiscardedEventDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	device := seedDevice(t, db, project.UID)

	t.Run("WithDiscarded", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.DiscardedEventStatus
			d.DeviceID = device.UID
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		results, err := service.FindDiscardedEventDeliveries(ctx, project.UID, device.UID, defaultSearchParams())
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 3)

		for _, d := range results {
			require.Equal(t, datastore.DiscardedEventStatus, d.Status)
		}
	})
}

func TestFindStuckEventDeliveriesByStatus(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("WithProcessing", func(t *testing.T) {
		ids := make([]string, 2)
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.ProcessingEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			ids[i] = d.UID
		}

		// The query requires created_at <= now() - 30s, so backdate them
		_, err := db.GetConn().Exec(ctx,
			`UPDATE convoy.event_deliveries SET created_at = NOW() - INTERVAL '1 minute' WHERE id = ANY($1)`, ids)
		require.NoError(t, err)

		results, err := service.FindStuckEventDeliveriesByStatus(ctx, datastore.ProcessingEventStatus)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 2)
	})
}

func TestUpdateEventDeliveryMetadata(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, delivery))

		nextSendTime := time.Now().Add(1 * time.Hour)
		delivery.Status = datastore.SuccessEventStatus
		delivery.Metadata = &datastore.Metadata{
			NumTrials:       3,
			RetryLimit:      5,
			IntervalSeconds: 120,
			NextSendTime:    nextSendTime,
		}
		delivery.LatencySeconds = 1.5

		err := service.UpdateEventDeliveryMetadata(ctx, project.UID, delivery)
		require.NoError(t, err)

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, datastore.SuccessEventStatus, found.Status)
		require.Equal(t, uint64(3), found.Metadata.NumTrials)
		require.Equal(t, uint64(5), found.Metadata.RetryLimit)
		require.Equal(t, uint64(120), found.Metadata.IntervalSeconds)
		require.InDelta(t, 1.5, found.LatencySeconds, 0.01)
	})
}

func TestCountEventDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("NoFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		count, err := service.CountEventDeliveries(ctx, project.UID, nil, "", nil, defaultSearchParams())
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(3))
	})

	t.Run("WithEndpointFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		sub2 := seedSubscription(t, db, project.UID, endpoint2.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		// 2 for endpoint1
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		// 1 for endpoint2
		d := createTestEventDelivery(t, project.UID, event.UID, endpoint2.UID, sub2.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, d))

		count, err := service.CountEventDeliveries(ctx, project.UID, []string{endpoint.UID}, "", nil, defaultSearchParams())
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})

	t.Run("WithStatusFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.SuccessEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		d.Status = datastore.FailureEventStatus
		require.NoError(t, service.CreateEventDelivery(ctx, d))

		count, err := service.CountEventDeliveries(ctx, project.UID, nil, "", []datastore.EventDeliveryStatus{datastore.SuccessEventStatus}, defaultSearchParams())
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})
}

func TestDeleteProjectEventDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("SoftDelete", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		ids := make([]string, 3)
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			ids[i] = d.UID
		}

		filter := &datastore.EventDeliveryFilter{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		}

		err := service.DeleteProjectEventDeliveries(ctx, project.UID, filter, false)
		require.NoError(t, err)
	})

	t.Run("HardDelete", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		filter := &datastore.EventDeliveryFilter{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		}

		err := service.DeleteProjectEventDeliveries(ctx, project.UID, filter, true)
		require.NoError(t, err)
	})
}

func TestLoadEventDeliveriesPaged(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("FirstPage_DESC", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 15; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 5, Direction: datastore.Next, Sort: "DESC"}
		deliveries, paginationData, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, deliveries, 5)
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
		require.True(t, paginationData.HasNextPage)

		for i := 1; i < len(deliveries); i++ {
			require.Greater(t, deliveries[i-1].UID, deliveries[i].UID)
		}
	})

	t.Run("FirstPage_ASC", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 15; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 5, Direction: datastore.Next, Sort: "ASC"}
		deliveries, paginationData, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, deliveries, 5)
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
		require.True(t, paginationData.HasNextPage)

		for i := 1; i < len(deliveries); i++ {
			require.Less(t, deliveries[i-1].UID, deliveries[i].UID)
		}
	})

	t.Run("NextPage", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 15; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 5, Direction: datastore.Next, Sort: "DESC"}
		page1, pagination1, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page1, 5)
		require.True(t, pagination1.HasNextPage)

		pageable.NextCursor = pagination1.NextPageCursor
		page2, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page2, 5)

		// No overlap
		page1IDs := make(map[string]bool)
		for _, d := range page1 {
			page1IDs[d.UID] = true
		}
		for _, d := range page2 {
			require.False(t, page1IDs[d.UID], "Page 2 should not contain page 1 IDs")
		}
	})

	t.Run("BackwardPagination", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 15; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 5, Direction: datastore.Next, Sort: "DESC"}
		page1, pagination1, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page1, 5)

		pageable.NextCursor = pagination1.NextPageCursor
		_, pagination2, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)

		// Go back
		pageable.Direction = datastore.Prev
		pageable.PrevCursor = pagination2.PrevPageCursor
		pageBack, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, pageBack, 5)

		for i := range page1 {
			require.Equal(t, page1[i].UID, pageBack[i].UID)
		}
	})

	t.Run("WithEndpointFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		sub2 := seedSubscription(t, db, project.UID, endpoint2.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		// 3 for endpoint1
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
		// 2 for endpoint2
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint2.UID, sub2.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
		deliveries, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, []string{endpoint.UID}, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, deliveries, 3)

		for _, d := range deliveries {
			require.Equal(t, endpoint.UID, d.EndpointID)
		}
	})

	t.Run("WithStatusFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.SuccessEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.FailureEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
		deliveries, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", []datastore.EventDeliveryStatus{datastore.SuccessEventStatus}, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, deliveries, 3)

		for _, d := range deliveries {
			require.Equal(t, datastore.SuccessEventStatus, d.Status)
		}
	})

	t.Run("WithSubscriptionFilter", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		sub2 := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
		for i := 0; i < 2; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub2.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
		deliveries, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", sub.UID, nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, deliveries, 3)

		for _, d := range deliveries {
			require.Equal(t, sub.UID, d.SubscriptionID)
		}
	})

	t.Run("EmptyResult", func(t *testing.T) {
		project := seedTestProject(t, db)

		pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
		deliveries, paginationData, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, []string{ulid.Make().String()}, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Empty(t, deliveries)
		require.False(t, paginationData.HasNextPage)
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
	})
}

func TestLoadEventDeliveriesIntervals(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	// Create some deliveries for interval data
	for i := 0; i < 3; i++ {
		d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, d))
	}

	t.Run("Daily", func(t *testing.T) {
		intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(intervals), minLen)
	})

	t.Run("Weekly", func(t *testing.T) {
		intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Weekly, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(intervals), minLen)
	})

	t.Run("Monthly", func(t *testing.T) {
		intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Monthly, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(intervals), minLen)
	})

	t.Run("Yearly", func(t *testing.T) {
		intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Yearly, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(intervals), minLen)
	})
}

func TestExportRecords(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	t.Run("Success", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		var buf bytes.Buffer
		// Export uses created_at < @created_at, so pass a future time to include recent deliveries
		count, err := service.ExportRecords(ctx, time.Now().Add(1*time.Hour), &buf)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(5))

		// Verify valid JSONL (one JSON object per line)
		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.GreaterOrEqual(t, len(lines), 5)
		for _, line := range lines {
			var record json.RawMessage
			err = json.Unmarshal(line, &record)
			require.NoError(t, err)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		_ = seedTestProject(t, db)

		var buf bytes.Buffer
		count, err := service.ExportRecords(ctx, time.Now().Add(1*time.Hour), &buf)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
		require.Empty(t, buf.String())
	})
}

func TestPartitionFunctions(t *testing.T) {
	service, _ := setupTestDB(t)
	ctx := context.Background()

	t.Run("PartitionEventDeliveriesTable", func(t *testing.T) {
		err := service.PartitionEventDeliveriesTable(ctx)
		require.NoError(t, err)
	})

	t.Run("UnPartitionEventDeliveriesTable", func(t *testing.T) {
		err := service.UnPartitionEventDeliveriesTable(ctx)
		require.NoError(t, err)
	})
}
