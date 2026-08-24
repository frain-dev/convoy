package event_deliveries

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
	_ = os.Setenv("CONVOY_JWT_SECRET", "test-access-secret")
	_ = os.Setenv("CONVOY_JWT_REFRESH_SECRET", "test-refresh-secret")

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
	return seedEventNamed(t, db, projectID, endpointID, sourceID, "test.event")
}

func seedEventNamed(t *testing.T, db database.Database, projectID, endpointID, sourceID, eventType string) *datastore.Event {
	t.Helper()

	logger := log.New("convoy", log.LevelInfo)
	ctx := context.Background()
	eventRepo := events.New(logger, db)

	eventID := ulid.Make().String()
	now := time.Now()
	event := &datastore.Event{
		UID:            eventID,
		EventType:      datastore.EventType(eventType),
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

		// Slim still loads description so write-backs do not clobber it.
		require.Equal(t, delivery.Description, found.Description)

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

	t.Run("PersistsDescription", func(t *testing.T) {
		delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, delivery))

		delivery.Status = datastore.FailureEventStatus
		delivery.Description = "Retry limit exceeded"
		delivery.Metadata = &datastore.Metadata{NumTrials: 5, RetryLimit: 5, IntervalSeconds: 1}

		require.NoError(t, service.UpdateEventDeliveryMetadata(ctx, project.UID, delivery))

		found, err := service.FindEventDeliveryByID(ctx, project.UID, delivery.UID)
		require.NoError(t, err)
		require.Equal(t, datastore.FailureEventStatus, found.Status)
		require.Equal(t, "Retry limit exceeded", found.Description)
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

func TestCountDeliveriesByEndpointAndStatus(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	endpoint2 := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	sub2 := seedSubscription(t, db, project.UID, endpoint2.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	seed := func(endpointID, subID string, status datastore.EventDeliveryStatus, n int) {
		for i := 0; i < n; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpointID, subID)
			d.Status = status
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
	}

	// endpoint1: 3 Success, 2 Failure, 1 Discarded (Discarded must be excluded).
	seed(endpoint.UID, sub.UID, datastore.SuccessEventStatus, 3)
	seed(endpoint.UID, sub.UID, datastore.FailureEventStatus, 2)
	seed(endpoint.UID, sub.UID, datastore.DiscardedEventStatus, 1)
	// endpoint2: 1 Success only.
	seed(endpoint2.UID, sub2.UID, datastore.SuccessEventStatus, 1)

	statuses := []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.FailureEventStatus}

	t.Run("CountsPerEndpointAndStatus", func(t *testing.T) {
		rows, err := service.CountDeliveriesByEndpointAndStatus(ctx, project.UID,
			[]string{endpoint.UID, endpoint2.UID}, statuses, defaultSearchParams())
		require.NoError(t, err)

		counts := map[string]map[datastore.EventDeliveryStatus]int64{}
		for _, r := range rows {
			if counts[r.EndpointID] == nil {
				counts[r.EndpointID] = map[datastore.EventDeliveryStatus]int64{}
			}
			counts[r.EndpointID][r.Status] = r.Count
		}

		require.Equal(t, int64(3), counts[endpoint.UID][datastore.SuccessEventStatus])
		require.Equal(t, int64(2), counts[endpoint.UID][datastore.FailureEventStatus])
		// Discarded is not in the requested status set, so it must not appear.
		require.Zero(t, counts[endpoint.UID][datastore.DiscardedEventStatus])
		require.Equal(t, int64(1), counts[endpoint2.UID][datastore.SuccessEventStatus])
		require.Zero(t, counts[endpoint2.UID][datastore.FailureEventStatus])
	})

	t.Run("EmptyEndpointIDs", func(t *testing.T) {
		rows, err := service.CountDeliveriesByEndpointAndStatus(ctx, project.UID, nil, statuses, defaultSearchParams())
		require.NoError(t, err)
		require.Empty(t, rows)
	})

	t.Run("RangeExcludesOlderDeliveries", func(t *testing.T) {
		// A window entirely in the past returns nothing for these just-created rows.
		past := datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-48 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(-24 * time.Hour).Unix(),
		}
		rows, err := service.CountDeliveriesByEndpointAndStatus(ctx, project.UID,
			[]string{endpoint.UID}, statuses, past)
		require.NoError(t, err)
		require.Empty(t, rows)
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
		page2, pagination2, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page2, 5)
		require.True(t, pagination2.HasPreviousPage)

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
		require.True(t, pagination2.HasPreviousPage)

		// Go back
		pageable.Direction = datastore.Prev
		pageable.PrevCursor = pagination2.PrevPageCursor
		pageBack, paginationBack, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, pageBack, 5)
		require.False(t, paginationBack.HasPreviousPage)

		for i := range page1 {
			require.Equal(t, page1[i].UID, pageBack[i].UID)
		}
	})

	t.Run("NextPageWhenCreatedAtAndIDDiverge", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		ids := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			ids = append(ids, d.UID)
			time.Sleep(2 * time.Millisecond)
		}

		_, err := db.GetConn().Exec(ctx,
			`UPDATE convoy.event_deliveries SET created_at = NOW() - INTERVAL '2 hours' WHERE id = $1 AND project_id = $2`,
			ids[2], project.UID)
		require.NoError(t, err)

		pageable := datastore.Pageable{PerPage: 1, Direction: datastore.Next, Sort: "DESC"}
		page1, pagination1, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page1, 1)
		require.Equal(t, ids[1], page1[0].UID)

		pageable.NextCursor = pagination1.NextPageCursor
		page2, pagination2, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page2, 1)
		require.Equal(t, ids[0], page2[0].UID)

		pageable.NextCursor = pagination2.NextPageCursor
		page3, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page3, 1)
		require.Equal(t, ids[2], page3[0].UID)
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

	t.Run("StatusNextPage", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		var successIDs []string
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = datastore.SuccessEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
			successIDs = append(successIDs, d.UID)
			time.Sleep(2 * time.Millisecond)
		}

		pageable := datastore.Pageable{PerPage: 1, Direction: datastore.Next, Sort: "DESC"}
		page1, pagination1, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", []datastore.EventDeliveryStatus{datastore.SuccessEventStatus}, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page1, 1)
		require.Equal(t, successIDs[2], page1[0].UID)
		require.True(t, pagination1.HasNextPage)

		pageable.NextCursor = pagination1.NextPageCursor
		page2, pagination2, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", []datastore.EventDeliveryStatus{datastore.SuccessEventStatus}, defaultSearchParams(), pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, page2, 1)
		require.True(t, pagination2.HasPreviousPage)
		require.Equal(t, successIDs[1], page2[0].UID)
		require.Equal(t, datastore.SuccessEventStatus, page2[0].Status)
	})

	t.Run("SearchByIDEventTypeAndEndpointName", func(t *testing.T) {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

		named := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		named.EventType = datastore.EventType("invoice.paid")
		require.NoError(t, service.CreateEventDelivery(ctx, named))

		other := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		other.EventType = datastore.EventType("charge.failed")
		require.NoError(t, service.CreateEventDelivery(ctx, other))

		pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}

		byID, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, datastore.SearchParams{
				CreatedAtStart: defaultSearchParams().CreatedAtStart,
				CreatedAtEnd:   defaultSearchParams().CreatedAtEnd,
				Query:          named.UID,
			}, pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, byID, 1)
		require.Equal(t, named.UID, byID[0].UID)

		byType, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, datastore.SearchParams{
				CreatedAtStart: defaultSearchParams().CreatedAtStart,
				CreatedAtEnd:   defaultSearchParams().CreatedAtEnd,
				Query:          "invoice",
			}, pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, byType, 1)
		require.Equal(t, named.UID, byType[0].UID)

		byEndpoint, _, err := service.LoadEventDeliveriesPaged(
			ctx, project.UID, nil, "", "", nil, datastore.SearchParams{
				CreatedAtStart: defaultSearchParams().CreatedAtStart,
				CreatedAtEnd:   defaultSearchParams().CreatedAtEnd,
				Query:          endpoint.Name,
			}, pageable, "", "", "",
		)
		require.NoError(t, err)
		require.Len(t, byEndpoint, 2)
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

func TestLoadEventDeliveriesIntervalsFromRollup(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	for i := 0; i < 3; i++ {
		d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, d))
	}

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), sumIntervalCounts(intervals))
}

func TestRefreshDailyCountsRewritesADayThatAlreadyHasRows(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	for i := 0; i < 3; i++ {
		require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))
	}

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))

	// The recent refresh runs every minute over the same two days, so the
	// second pass carries the primary key the first one just wrote.
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))

	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(4), sumIntervalCounts(intervals))
}

func TestRefreshDailyCountsDropsKeysThatNoLongerAggregate(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.Equal(t, 1, rollupRowCount(t, db, project.UID))

	_, err := db.GetDB().ExecContext(ctx,
		"UPDATE convoy.event_deliveries SET deleted_at=NOW() WHERE id=$1", d.UID)
	require.NoError(t, err)

	// An upsert alone would leave the row at its stale count.
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.Equal(t, 0, rollupRowCount(t, db, project.UID))
}

func rollupRowCount(t *testing.T, db database.Database, projectID string) int {
	t.Helper()
	var count int
	require.NoError(t, db.GetDB().QueryRowxContext(context.Background(),
		"SELECT COUNT(*) FROM convoy.event_delivery_daily_counts WHERE project_id=$1",
		projectID).Scan(&count))
	return count
}

func TestLoadEventDeliveriesIntervalsFromRollupExcludesEndMidnight(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	day := time.Date(2022, time.March, 20, 0, 0, 0, 0, time.UTC)
	noon := day.Add(12 * time.Hour)
	d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, d))
	_, err := db.GetDB().ExecContext(ctx,
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2",
		noon, d.UID)
	require.NoError(t, err)

	midnight := datastore.SearchParams{CreatedAtStart: day.Unix(), CreatedAtEnd: day.Unix()}
	live, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, midnight, datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), sumIntervalCounts(live))

	require.NoError(t, service.RefreshDailyCounts(ctx, day, day.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	rollup, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, midnight, datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), sumIntervalCounts(rollup))

	throughNoon := datastore.SearchParams{CreatedAtStart: day.Unix(), CreatedAtEnd: noon.Unix()}
	included, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, throughNoon, datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sumIntervalCounts(included))
}

func TestLoadEventDeliveriesIntervalsFromRollupEndpointFilter(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpointA := seedTestEndpoint(t, db, project.UID)
	endpointB := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subA := seedSubscription(t, db, project.UID, endpointA.UID, source.UID)
	subB := seedSubscription(t, db, project.UID, endpointB.UID, source.UID)
	eventA := seedEvent(t, db, project.UID, endpointA.UID, source.UID)
	eventB := seedEvent(t, db, project.UID, endpointB.UID, source.UID)

	for i := 0; i < 2; i++ {
		require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, eventA.UID, endpointA.UID, subA.UID)))
	}
	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, eventB.UID, endpointB.UID, subB.UID)))

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	all, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), sumIntervalCounts(all))

	filtered, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, []string{endpointA.UID})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sumIntervalCounts(filtered))
}

func TestLoadEventIntervalsCountsEventsNotDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpointA := seedTestEndpoint(t, db, project.UID)
	endpointB := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subA := seedSubscription(t, db, project.UID, endpointA.UID, source.UID)
	subB := seedSubscription(t, db, project.UID, endpointB.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpointA.UID, source.UID)
	_, err := db.GetDB().ExecContext(ctx,
		"INSERT INTO convoy.events_endpoints (event_id, endpoint_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		event.UID, endpointB.UID)
	require.NoError(t, err)

	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpointA.UID, subA.UID)))
	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpointB.UID, subB.UID)))

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))
	require.NoError(t, service.markEventDailyCountsBackfillCompleted(ctx))

	projectIntervals, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sumIntervalCounts(projectIntervals))

	deliveries, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), sumIntervalCounts(deliveries))

	portalA, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, []string{endpointA.UID})
	require.NoError(t, err)
	require.Equal(t, uint64(1), sumIntervalCounts(portalA))

	both, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, []string{endpointA.UID, endpointB.UID})
	require.NoError(t, err)
	require.Equal(t, uint64(1), sumIntervalCounts(both))

	event2 := seedEvent(t, db, project.UID, endpointA.UID, source.UID)
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	after, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), sumIntervalCounts(after))

	unmatched := seedEvent(t, db, project.UID, endpointA.UID, source.UID)
	_, err = db.GetDB().ExecContext(ctx, "DELETE FROM convoy.events_endpoints WHERE event_id = $1", unmatched.UID)
	require.NoError(t, err)
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	withUnmatched, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), sumIntervalCounts(withUnmatched))
	portalAfterUnmatched, err := service.LoadEventIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, []string{endpointA.UID})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sumIntervalCounts(portalAfterUnmatched))
	_ = event2
}

func TestLoadEventDeliveriesIntervalsFromRollupWeekly(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	for i := 0; i < 3; i++ {
		require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))
	}

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Weekly, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), sumIntervalCounts(intervals))
}

func TestWriteQueueMetricsSnapshotRoundTrip(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
	d.Status = datastore.SuccessEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	pg, ok := db.(*postgres.Postgres)
	require.True(t, ok)
	require.NoError(t, pg.WriteQueueMetricsSnapshot(ctx))

	var gen int64
	var total int64
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT m.generation, COALESCE(SUM(q.total), 0)
		FROM convoy.metrics_snapshot_meta m
		LEFT JOIN convoy.metrics_event_delivery_queue q
		  ON q.generation = m.generation
		WHERE m.name = 'event_delivery_queue'
		GROUP BY m.generation`).Scan(&gen, &total))
	require.Equal(t, int64(1), gen)
	require.GreaterOrEqual(t, total, int64(1))

	require.NoError(t, pg.WriteQueueMetricsSnapshot(ctx))
	var leftover int64
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.metrics_event_delivery_queue WHERE generation <> 2`).Scan(&leftover))
	require.Zero(t, leftover)
}

func sumIntervalCounts(intervals []datastore.EventInterval) uint64 {
	var total uint64
	for _, in := range intervals {
		total += in.Count
	}
	return total
}

func TestPruneDailyCountsBeforeLiveHistoryDropsRetainedDays(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))

	today := utcDate(time.Now())
	require.NoError(t, service.RefreshDailyCounts(ctx, today, today.AddDate(0, 0, 1)))
	_, err := db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, status, count)
		VALUES ($1, $2, '2020-01-01', 'Success', 99)`, project.UID, endpoint.UID)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_daily_counts (project_id, day, count)
		VALUES ($1, '2020-01-01', 99)`, project.UID)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_endpoint_daily_counts (project_id, endpoint_id, day, count)
		VALUES ($1, $2, '2020-01-01', 99)`, project.UID, endpoint.UID)
	require.NoError(t, err)

	require.NoError(t, service.PruneDailyCountsBeforeLiveHistory(ctx))

	var stale int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_delivery_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Zero(t, stale)
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Zero(t, stale)
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_endpoint_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Zero(t, stale)

	var kept int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_delivery_daily_counts
		WHERE project_id = $1 AND day = $2`, project.UID, today).Scan(&kept))
	require.Equal(t, 1, kept)
}

func TestMaybePruneEventDailyCountsRunsWhenDeliveryPruneSkips(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	_, err := db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL, last_pruned_at = NOW()
		WHERE name = 'backfill'`)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL, last_pruned_at = NULL
		WHERE name = 'events_backfill'`)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_daily_counts (project_id, day, count)
		VALUES ($1, '2020-01-01', 99)`, project.UID)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_endpoint_daily_counts (project_id, endpoint_id, day, count)
		VALUES ($1, $2, '2020-01-01', 99)`, project.UID, endpoint.UID)
	require.NoError(t, err)

	require.NoError(t, service.maybePruneDailyCountsBeforeLiveHistory(ctx))
	var stale int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Equal(t, 1, stale, "delivery prune skip must leave the event rollup untouched")

	require.NoError(t, service.RefreshRecentDailyCounts(ctx))
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Zero(t, stale)
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_endpoint_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Zero(t, stale)
}

func TestAdvanceEventDailyCountsBackfillDoesNotRewriteDeliveries(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	day := utcDate(time.Now()).AddDate(0, 0, -3)
	dayKey := day.Format("2006-01-02")
	noon := day.Add(12 * time.Hour)
	_, err := db.GetDB().ExecContext(ctx,
		"UPDATE convoy.events SET created_at=$1, updated_at=$1 WHERE id=$2",
		noon, event.UID)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx,
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2",
		noon, d.UID)
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL
		WHERE name = 'backfill'`)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NULL, next_day = $1
		WHERE name = 'events_backfill'`, dayKey)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, status, count)
		VALUES ($1, $2, $3, 'Success', 999)`, project.UID, endpoint.UID, dayKey)
	require.NoError(t, err)

	_, err = service.AdvanceEventDailyCountsBackfill(ctx)
	require.NoError(t, err)

	var deliveryCount int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT count FROM convoy.event_delivery_daily_counts
		WHERE project_id = $1 AND endpoint_id = $2 AND day = $3 AND status = 'Success'`,
		project.UID, endpoint.UID, dayKey).Scan(&deliveryCount))
	require.Equal(t, 999, deliveryCount)

	var eventCount int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT count FROM convoy.event_daily_counts
		WHERE project_id = $1 AND day = $2`, project.UID, dayKey).Scan(&eventCount))
	require.Equal(t, 1, eventCount)
}

func TestMaybePruneDailyCountsSkipsWithin24Hours(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	_, err := db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL, last_pruned_at = NOW()
		WHERE name = 'backfill'`)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, status, count)
		VALUES ($1, $2, '2020-01-01', 'Success', 99)`, project.UID, endpoint.UID)
	require.NoError(t, err)

	require.NoError(t, service.maybePruneDailyCountsBeforeLiveHistory(ctx))

	var stale int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.event_delivery_daily_counts
		WHERE project_id = $1 AND day = '2020-01-01'`, project.UID).Scan(&stale))
	require.Equal(t, 1, stale)
}

func TestLoadEventDeliveriesIntervalsFromRollupWeeklyIgnoresSessionTimeZone(t *testing.T) {
	_, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	monday := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	_, err := db.GetDB().ExecContext(ctx, `
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, status, count)
		VALUES ($1, 'ep', $2, 'Success', 4)`, project.UID, monday)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL
		WHERE name = 'backfill'`)
	require.NoError(t, err)

	query, err := rollupIntervalQuery(datastore.Weekly)
	require.NoError(t, err)

	tx, err := db.GetDB().BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "SET LOCAL TIME ZONE 'America/Los_Angeles'")
	require.NoError(t, err)

	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
	var r intervalRow
	require.NoError(t, tx.QueryRowContext(ctx, query, project.UID, start, end, false, []string{}).Scan(&r.DataIndex, &r.DataTotalTime, &r.Count))
	require.Equal(t, "2026-08-17", r.DataTotalTime.String)
	require.Equal(t, int64(4), r.Count.Int64)
}

func TestAdvanceDailyCountsBackfillCompletesWhenEmpty(t *testing.T) {
	service, _ := setupTestDB(t)
	ctx := context.Background()

	done, err := service.AdvanceDailyCountsBackfill(ctx)
	require.NoError(t, err)
	require.True(t, done)

	completed, err := service.dailyCountsBackfillCompleted(ctx)
	require.NoError(t, err)
	require.True(t, completed)
}

// The worker calls this every minute over the same two days, so the second and
// every later run operate on a rollup that already holds those days. Tests that
// only ever refresh a pristine table never reach that state.
func TestRefreshRecentDailyCountsRepeatsLikeTheWorker(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))

	for i := 0; i < 3; i++ {
		require.NoError(t, service.RefreshRecentDailyCounts(ctx))
	}

	require.NoError(t, service.CreateEventDelivery(ctx, createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)))
	require.NoError(t, service.RefreshRecentDailyCounts(ctx))
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, defaultSearchParams(), datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(2), sumIntervalCounts(intervals))
}

func TestAdvanceDailyCountsBackfillWalksHistoryToCompletion(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	today := utcDate(time.Now())
	for i := 1; i <= 3; i++ {
		d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
		require.NoError(t, service.CreateEventDelivery(ctx, d))
		at := today.AddDate(0, 0, -i).Add(12 * time.Hour)
		_, err := db.GetDB().ExecContext(ctx,
			"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2", at, d.UID)
		require.NoError(t, err)
	}

	// Each step refreshes one day, and the recent window re-covers the last of
	// them, so the walk crosses days that the rollup already holds.
	done := false
	for i := 0; i < 10 && !done; i++ {
		require.NoError(t, service.RefreshRecentDailyCounts(ctx))
		var err error
		done, err = service.AdvanceDailyCountsBackfill(ctx)
		require.NoError(t, err)
	}
	require.True(t, done, "backfill did not complete")

	window := datastore.SearchParams{
		CreatedAtStart: today.AddDate(0, 0, -4).Unix(),
		CreatedAtEnd:   today.AddDate(0, 0, 1).Unix(),
	}
	intervals, err := service.LoadEventDeliveriesIntervals(ctx, project.UID, window, datastore.Daily, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), sumIntervalCounts(intervals))
	require.Equal(t, 3, rollupRowCount(t, db, project.UID))
}

// The dashboard's Successful/Failed cards read these totals. They used to come
// from one live COUNT(*) per status, and a timeout on either rendered as a
// confident zero, so the rollup has to reproduce the live numbers per status.
func TestStatusTotalsAgreePerStatusAcrossSources(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	src := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, endpoint.UID, src.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, src.UID)

	seed := func(status datastore.EventDeliveryStatus, n int) {
		for i := 0; i < n; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			d.Status = status
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
	}
	seed(datastore.SuccessEventStatus, 3)
	seed(datastore.FailureEventStatus, 2)

	// Until the backfill completes the totals come off a live scan, which is the
	// number the rollup then has to match.
	live, liveSource, err := service.StatusTotals(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, StatusTotalsFromLive, liveSource)
	require.Equal(t, int64(3), live[datastore.SuccessEventStatus])
	require.Equal(t, int64(2), live[datastore.FailureEventStatus])

	// A status with no deliveries is absent rather than zero, so the caller can
	// tell an empty window from a failed request.
	_, present := live[datastore.DiscardedEventStatus]
	require.False(t, present, "a status with no deliveries must not be reported as 0")

	// Twice, because the worker rewrites the same days every minute and the
	// second pass is the one carrying keys the first pass already wrote.
	for i := 0; i < 2; i++ {
		require.NoError(t, service.RefreshRecentDailyCounts(ctx))
	}
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	fromRollup, rollupSource, err := service.StatusTotals(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, StatusTotalsFromRollup, rollupSource)
	require.Equal(t, live, fromRollup, "rollup disagrees with the live scan")

	// A delivery landing after the rollup was first written must show up on the
	// next refresh, which only holds if a populated day can be rewritten.
	seed(datastore.SuccessEventStatus, 1)
	require.NoError(t, service.RefreshRecentDailyCounts(ctx))

	updated, _, err := service.StatusTotals(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, int64(4), updated[datastore.SuccessEventStatus])
	require.Equal(t, int64(2), updated[datastore.FailureEventStatus])
}

func TestStatusTotalsScopeToEndpoint(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	scoped := seedTestEndpoint(t, db, project.UID)
	other := seedTestEndpoint(t, db, project.UID)
	src := seedTestSource(t, db, project.UID)
	event := seedEvent(t, db, project.UID, scoped.UID, src.UID)

	seed := func(endpointID string, n int) {
		sub := seedSubscription(t, db, project.UID, endpointID, src.UID)
		for i := 0; i < n; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpointID, sub.UID)
			d.Status = datastore.SuccessEventStatus
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
	}
	seed(scoped.UID, 2)
	seed(other.UID, 5)

	for i := 0; i < 2; i++ {
		require.NoError(t, service.RefreshRecentDailyCounts(ctx))
	}
	require.NoError(t, service.markDailyCountsBackfillCompleted(ctx))

	totals, source, err := service.StatusTotals(ctx, project.UID, defaultSearchParams(), []string{scoped.UID})
	require.NoError(t, err)
	require.Equal(t, StatusTotalsFromRollup, source)
	require.Equal(t, int64(2), totals[datastore.SuccessEventStatus])

	all, _, err := service.StatusTotals(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, int64(7), all[datastore.SuccessEventStatus])
}

// staleDays reads the days the writers flagged for the per-status rollup.
func staleDays(t *testing.T, db database.Database) []time.Time {
	t.Helper()

	rows, err := db.GetConn().Query(context.Background(),
		`SELECT day FROM convoy.event_delivery_daily_counts_stale ORDER BY day`)
	require.NoError(t, err)
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var day pgtype.Date
		require.NoError(t, rows.Scan(&day))
		days = append(days, day.Time)
	}
	require.NoError(t, rows.Err())
	return days
}

// closedDayFixture is a project with deliveries seeded on days that have left
// the refresh window, which is where per-status drift shows up.
type closedDayFixture struct {
	service  *Service
	db       database.Database
	project  *datastore.Project
	endpoint *datastore.Endpoint
	sub      *datastore.Subscription
	event    *datastore.Event
}

func newClosedDayFixture(t *testing.T) *closedDayFixture {
	t.Helper()

	service, db := setupTestDB(t)
	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	src := seedTestSource(t, db, project.UID)

	return &closedDayFixture{
		service:  service,
		db:       db,
		project:  project,
		endpoint: endpoint,
		sub:      seedSubscription(t, db, project.UID, endpoint.UID, src.UID),
		event:    seedEvent(t, db, project.UID, endpoint.UID, src.UID),
	}
}

func (f *closedDayFixture) seedOn(t *testing.T, day time.Time, status datastore.EventDeliveryStatus) *datastore.EventDelivery {
	t.Helper()
	ctx := context.Background()

	d := createTestEventDelivery(t, f.project.UID, f.event.UID, f.endpoint.UID, f.sub.UID)
	d.Status = status
	require.NoError(t, f.service.CreateEventDelivery(ctx, d))

	at := day.Add(12 * time.Hour)
	_, err := f.db.GetConn().Exec(ctx,
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2", at, d.UID)
	require.NoError(t, err)

	d.CreatedAt = at
	return d
}

// rollForward brings the rollup up to date for every seeded day and closes the
// backfill, so later reads come from the rollup rather than a live scan.
func (f *closedDayFixture) rollForward(t *testing.T, days ...time.Time) {
	t.Helper()
	ctx := context.Background()

	for _, day := range days {
		require.NoError(t, f.service.RefreshDailyCounts(ctx, day, day.AddDate(0, 0, 1)))
	}
	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.NoError(t, f.service.markDailyCountsBackfillCompleted(ctx))
}

func (f *closedDayFixture) totals(t *testing.T) map[datastore.EventDeliveryStatus]int64 {
	t.Helper()

	params := datastore.SearchParams{
		CreatedAtStart: time.Now().AddDate(0, 0, -30).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	}
	totals, source, err := f.service.StatusTotals(context.Background(), f.project.UID, params, nil)
	require.NoError(t, err)
	require.Equal(t, StatusTotalsFromRollup, source)
	return totals
}

func TestStatusTotalsFollowARetryOnAClosedDay(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	// Five days back: past the window the refresh rewrites, so nothing but the
	// stale marker can bring this day's split up to date.
	day := utcDate(time.Now()).AddDate(0, 0, -5)
	retried := f.seedOn(t, day, datastore.FailureEventStatus)
	f.seedOn(t, day, datastore.FailureEventStatus)
	f.rollForward(t, day)

	require.Equal(t, int64(2), f.totals(t)[datastore.FailureEventStatus])
	require.Empty(t, staleDays(t, f.db), "seeding alone must not flag a day")

	require.NoError(t, f.service.UpdateStatusOfEventDelivery(ctx, f.project.UID, *retried, datastore.SuccessEventStatus))
	require.Equal(t, []time.Time{day}, staleDays(t, f.db), "the retried day was not flagged")

	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	after := f.totals(t)
	require.Equal(t, int64(1), after[datastore.FailureEventStatus])
	require.Equal(t, int64(1), after[datastore.SuccessEventStatus])
	require.Empty(t, staleDays(t, f.db), "the drain left its own marker behind")

	// Second pass over a day the first pass already rewrote, which is what the
	// every-minute job actually does.
	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.Equal(t, after, f.totals(t))
}

func TestForceResendFlagsEveryClosedDayItTouches(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	older := utcDate(time.Now()).AddDate(0, 0, -9)
	newer := utcDate(time.Now()).AddDate(0, 0, -3)
	first := f.seedOn(t, older, datastore.FailureEventStatus)
	second := f.seedOn(t, newer, datastore.FailureEventStatus)
	f.rollForward(t, older, newer)

	require.Equal(t, int64(2), f.totals(t)[datastore.FailureEventStatus])

	// One statement spanning both days, the shape force resend and batch retry
	// use. A marker for only the day in the diff would leave the other adrift.
	require.NoError(t, f.service.UpdateStatusOfEventDeliveries(ctx, f.project.UID,
		[]string{first.UID, second.UID}, datastore.SuccessEventStatus))
	require.Equal(t, []time.Time{older, newer}, staleDays(t, f.db))

	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	after := f.totals(t)
	require.Equal(t, int64(2), after[datastore.SuccessEventStatus])
	_, present := after[datastore.FailureEventStatus]
	require.False(t, present, "both closed days should have been rewritten")
}

func TestStatusChangeInsideTheWindowFlagsNothing(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	// Today is rewritten by every run regardless, so the delivery hot path must
	// not pay for a marker.
	d := createTestEventDelivery(t, f.project.UID, f.event.UID, f.endpoint.UID, f.sub.UID)
	require.NoError(t, f.service.CreateEventDelivery(ctx, d))
	require.NoError(t, f.service.UpdateStatusOfEventDelivery(ctx, f.project.UID, *d, datastore.SuccessEventStatus))

	require.Empty(t, staleDays(t, f.db))
}

func TestStaleDayDrainIsBoundedAndResumes(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	// Two days more than one run may drain, all of them clear of the window.
	const closedDays = staleDailyCountsDrainLimit + 2

	var days []time.Time
	var ids []string
	for i := closedDays; i >= 1; i-- {
		day := utcDate(time.Now()).AddDate(0, 0, -(i + 2))
		days = append(days, day)
		ids = append(ids, f.seedOn(t, day, datastore.FailureEventStatus).UID)
	}
	f.rollForward(t, days...)

	require.NoError(t, f.service.UpdateStatusOfEventDeliveries(ctx, f.project.UID, ids, datastore.SuccessEventStatus))
	require.Len(t, staleDays(t, f.db), len(days))

	// A resend spanning months must not hold the job for as long as rewriting
	// every day it touched takes, so one run drains a bounded slice.
	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.Equal(t, days[staleDailyCountsDrainLimit:], staleDays(t, f.db), "the oldest days should have drained first")

	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.Empty(t, staleDays(t, f.db))
	require.Equal(t, int64(len(days)), f.totals(t)[datastore.SuccessEventStatus])
}

func TestRefreshCatchesUpDaysTheWindowSkipped(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	today := utcDate(time.Now())
	const outageDays = 5
	for i := outageDays; i >= 1; i-- {
		f.seedOn(t, today.AddDate(0, 0, -i), datastore.FailureEventStatus)
	}
	d := createTestEventDelivery(t, f.project.UID, f.event.UID, f.endpoint.UID, f.sub.UID)
	d.Status = datastore.FailureEventStatus
	require.NoError(t, f.service.CreateEventDelivery(ctx, d))

	// The worker last ran five days ago and the API kept ingesting. Its two-day
	// window cannot reach those days, and with the backfill closed nothing else
	// revisits them, so they would stay missing from the rollup for good.
	_, err := f.db.GetConn().Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET last_refreshed_day = $1, completed_at = NOW(), next_day = NULL
		WHERE name = 'backfill'`, today.AddDate(0, 0, -outageDays))
	require.NoError(t, err)

	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.Equal(t, int64(outageDays+1), f.totals(t)[datastore.FailureEventStatus],
		"days the window skipped were never rolled up")
	require.Empty(t, staleDays(t, f.db))

	var watermark pgtype.Date
	require.NoError(t, f.db.GetConn().QueryRow(ctx, `
		SELECT last_refreshed_day FROM convoy.event_delivery_daily_counts_meta
		WHERE name = 'backfill'`).Scan(&watermark))
	require.True(t, watermark.Valid)
	require.Equal(t, today, utcDate(watermark.Time))

	// A second run in the same day has no gap to close and must not requeue.
	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.Empty(t, staleDays(t, f.db))
	require.Equal(t, int64(outageDays+1), f.totals(t)[datastore.FailureEventStatus])
}

func TestRefreshWatermarkStaysUntilRefreshSucceeds(t *testing.T) {
	f := newClosedDayFixture(t)
	ctx := context.Background()

	today := utcDate(time.Now())
	old := today.AddDate(0, 0, -5)
	_, err := f.db.GetConn().Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET last_refreshed_day = $1, completed_at = NOW(), next_day = NULL
		WHERE name = 'backfill'`, old)
	require.NoError(t, err)

	require.NoError(t, f.service.queueDaysTheWindowSkipped(ctx, today))

	var watermark pgtype.Date
	require.NoError(t, f.db.GetConn().QueryRow(ctx, `
		SELECT last_refreshed_day FROM convoy.event_delivery_daily_counts_meta
		WHERE name = 'backfill'`).Scan(&watermark))
	require.True(t, watermark.Valid)
	require.Equal(t, old, utcDate(watermark.Time), "queueing alone must not advance the watermark")

	require.NoError(t, f.service.RefreshRecentDailyCounts(ctx))
	require.NoError(t, f.db.GetConn().QueryRow(ctx, `
		SELECT last_refreshed_day FROM convoy.event_delivery_daily_counts_meta
		WHERE name = 'backfill'`).Scan(&watermark))
	require.Equal(t, today, utcDate(watermark.Time))
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
		// Export uses created_at < end AND created_at >= start, so pass epoch as start and future time as end
		count, err := service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now().Add(1*time.Hour), &buf)
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

	t.Run("Empty_with_past_cutoff", func(t *testing.T) {
		// Export with end in the past should return 0 records
		var buf bytes.Buffer
		count, err := service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Now().Add(-24*time.Hour), &buf)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
		require.Empty(t, buf.String())
	})

	t.Run("TimeWindow", func(t *testing.T) {
		// Create more deliveries to ensure we have records
		for i := 0; i < 3; i++ {
			d := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, sub.UID)
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}

		// Export with a narrow window: [1 hour ago, now+1h)
		// Should include all recently created deliveries
		var buf bytes.Buffer
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(1 * time.Hour)
		count, err := service.ExportRecords(ctx, start, end, &buf)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(3))

		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.GreaterOrEqual(t, len(lines), 3)

		// Export with a window that excludes all records: [2h ago, 1h ago)
		buf.Reset()
		count, err = service.ExportRecords(ctx, time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour), &buf)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
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

// Partitioning must not drop event-id enforcement. Both orders of the two
// partition commands are reachable from `convoy utils partition <table>`, and
// the state of convoy.events differs between them.
//
// Attaching enforces with the trigger either way, where the copy path used a
// real foreign key when events was unpartitioned. A validated foreign key on a
// partitioned parent scans every partition while holding SHARE ROW EXCLUSIVE,
// and Postgres 16 rejects NOT VALID here, so the foreign key would block writes
// for as long as the scan takes. Both cases are kept because both are still
// reachable and both must reject an orphan.
//
// Asserting the behaviour rather than the catalog: what matters is that a
// delivery naming an event that does not exist is rejected, not which mechanism
// rejects it.
func TestPartitionEventDeliveriesTableKeepsEventIDEnforcement(t *testing.T) {
	for _, tc := range []struct {
		name            string
		wantMechanism   string
		partitionEvents bool
	}{
		{name: "events partitioned first", wantMechanism: "event_fk_check trigger", partitionEvents: true},
		{name: "events left unpartitioned", wantMechanism: "event_fk_check trigger", partitionEvents: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, db := setupTestDB(t)
			ctx := context.Background()

			project := seedTestProject(t, db)
			endpoint := seedTestEndpoint(t, db, project.UID)
			source := seedTestSource(t, db, project.UID)
			subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
			event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

			// The partition helpers only build children for days that hold rows.
			require.NoError(t, service.CreateEventDelivery(ctx,
				createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))

			if tc.partitionEvents {
				require.NoError(t, events.New(log.New("convoy", log.LevelInfo), db).PartitionEventsTable(ctx))
			}
			require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

			orphan := createTestEventDelivery(t, project.UID, ulid.Make().String(), endpoint.UID, subscription.UID)
			require.Error(t, service.CreateEventDelivery(ctx, orphan),
				"delivery referencing a nonexistent event was accepted: %s is missing", tc.wantMechanism)

			// A valid delivery must still be accepted, so the test cannot pass on a
			// mechanism that rejects everything.
			require.NoError(t, service.CreateEventDelivery(ctx,
				createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))
		})
	}
}

// UnPartition rebuilds convoy.event_deliveries too, so it owes the same
// event-id enforcement the partition path owes. Unpartitioning deliveries says
// nothing about convoy.events, which may still be partitioned.
func TestUnPartitionEventDeliveriesTableKeepsEventIDEnforcement(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	require.NoError(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))

	// events stays partitioned, so the trigger is the only form available.
	require.NoError(t, events.New(log.New("convoy", log.LevelInfo), db).PartitionEventsTable(ctx))
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))
	require.NoError(t, service.UnPartitionEventDeliveriesTable(ctx))

	orphan := createTestEventDelivery(t, project.UID, ulid.Make().String(), endpoint.UID, subscription.UID)
	require.Error(t, service.CreateEventDelivery(ctx, orphan),
		"delivery referencing a nonexistent event was accepted after unpartitioning")

	require.NoError(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))
}

// Partitions must be named so retention can adopt them. See
// testenv.RequirePartitionsAddressableByRetention for why that is not automatic.
func TestPartitionEventDeliveriesTableNamesForRetention(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, delivery))

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))
	testenv.RequirePartitionsAddressableByRetention(t, db, "event_deliveries", project.UID)
}

func TestObservedEventTypes(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	other := seedTestProject(t, db)
	ep1 := seedTestEndpoint(t, db, project.UID)
	ep2 := seedTestEndpoint(t, db, project.UID)
	otherEp := seedTestEndpoint(t, db, other.UID)
	src := seedTestSource(t, db, project.UID)
	otherSrc := seedTestSource(t, db, other.UID)
	sub1 := seedSubscription(t, db, project.UID, ep1.UID, src.UID)
	sub2 := seedSubscription(t, db, project.UID, ep2.UID, src.UID)
	otherSub := seedSubscription(t, db, other.UID, otherEp.UID, otherSrc.UID)
	eventPaid := seedEventNamed(t, db, project.UID, ep1.UID, src.UID, "invoice.paid")
	eventOrder := seedEventNamed(t, db, project.UID, ep2.UID, src.UID, "order.created")
	eventStar := seedEventNamed(t, db, project.UID, ep1.UID, src.UID, "*")
	eventOld := seedEventNamed(t, db, project.UID, ep1.UID, src.UID, "old.event")
	otherEvent := seedEventNamed(t, db, other.UID, otherEp.UID, otherSrc.UID, "webhook.received")

	createFor := func(projectID, eventID, endpointID, subID string, eventType datastore.EventType) *datastore.EventDelivery {
		t.Helper()
		d := createTestEventDelivery(t, projectID, eventID, endpointID, subID)
		d.EventType = eventType
		require.NoError(t, service.CreateEventDelivery(ctx, d))
		return d
	}

	createFor(project.UID, eventPaid.UID, ep1.UID, sub1.UID, eventPaid.EventType)
	createFor(project.UID, eventPaid.UID, ep1.UID, sub1.UID, eventPaid.EventType)
	createFor(project.UID, eventStar.UID, ep1.UID, sub1.UID, eventStar.EventType)
	createFor(project.UID, eventOrder.UID, ep2.UID, sub2.UID, eventOrder.EventType)
	old := createFor(project.UID, eventOld.UID, ep1.UID, sub1.UID, eventOld.EventType)
	createFor(other.UID, otherEvent.UID, otherEp.UID, otherSub.UID, otherEvent.EventType)

	_, err := db.GetConn().Exec(ctx,
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2",
		time.Now().Add(-48*time.Hour), old.UID)
	require.NoError(t, err)

	names, err := service.ObservedEventTypes(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"invoice.paid", "order.created"}, names)

	scoped, err := service.ObservedEventTypes(ctx, project.UID, defaultSearchParams(), []string{ep1.UID})
	require.NoError(t, err)
	require.Equal(t, []string{"invoice.paid"}, scoped)

	catalog, observed := GroupFilterEventTypes([]datastore.ProjectEventType{
		{Name: "invoice.paid"},
		{Name: "*"},
	}, names)
	require.Equal(t, []string{"invoice.paid"}, catalog)
	require.Equal(t, []string{"order.created"}, observed)
}

func TestObservedEventTypesReadsDeliveryEventType(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	ep := seedTestEndpoint(t, db, project.UID)
	src := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, ep.UID, src.UID)
	event := seedEventNamed(t, db, project.UID, ep.UID, src.UID, "bench.event")

	d := createTestEventDelivery(t, project.UID, event.UID, ep.UID, sub.UID)
	d.EventType = event.EventType
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
	listed, _, err := service.LoadEventDeliveriesPaged(
		ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
	)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "bench.event", string(listed[0].EventType))

	observed, err := service.ObservedEventTypes(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"bench.event"}, observed)

	observedAgain, err := service.ObservedEventTypes(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Equal(t, observed, observedAgain)

	filtered, _, err := service.LoadEventDeliveriesPaged(
		ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "bench.event", "",
	)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, d.UID, filtered[0].UID)

	catalog, grouped := GroupFilterEventTypes(nil, observed)
	require.Empty(t, catalog)
	require.Equal(t, []string{"bench.event"}, grouped)
}

func TestObservedEventTypesSkipsBlankDeliveryType(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	ep := seedTestEndpoint(t, db, project.UID)
	src := seedTestSource(t, db, project.UID)
	sub := seedSubscription(t, db, project.UID, ep.UID, src.UID)
	event := seedEventNamed(t, db, project.UID, ep.UID, src.UID, "bench.event")

	d := createTestEventDelivery(t, project.UID, event.UID, ep.UID, sub.UID)
	d.EventType = ""
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, Sort: "DESC"}
	listed, _, err := service.LoadEventDeliveriesPaged(
		ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "", "",
	)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].Event)
	require.Equal(t, "bench.event", string(listed[0].Event.EventType))
	require.Empty(t, string(listed[0].EventType))

	observed, err := service.ObservedEventTypes(ctx, project.UID, defaultSearchParams(), nil)
	require.NoError(t, err)
	require.Empty(t, observed)

	filtered, _, err := service.LoadEventDeliveriesPaged(
		ctx, project.UID, nil, "", "", nil, defaultSearchParams(), pageable, "", "bench.event", "",
	)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, d.UID, filtered[0].UID)
}
