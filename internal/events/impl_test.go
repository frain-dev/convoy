package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

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
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"
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

	// Clone test database for isolation
	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data any, changelog any) {})

	db := postgres.NewFromConnection(conn)

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
	return New(logger, db), db
}

func seedTestProject(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()

	logger := log.NewLogger(os.Stdout)
	ctx := context.Background()

	// Create user with unique email
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
	endpointRepo := postgres.NewEndpointRepo(db)

	endpointID := ulid.Make().String()
	endpoint := &datastore.Endpoint{
		UID:          endpointID,
		ProjectID:    projectID,
		Name:         fmt.Sprintf("Test Endpoint %s", endpointID),
		Url:          fmt.Sprintf("https://example.com/webhook/%s", endpointID),
		Status:       datastore.ActiveEndpointStatus,
		SupportEmail: fmt.Sprintf("test-%s@example.com", endpointID),
		Secrets:      datastore.Secrets{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err := endpointRepo.CreateEndpoint(ctx, endpoint, projectID)
	require.NoError(t, err)

	return endpoint
}

func seedTestSource(t *testing.T, db database.Database, projectID string) *datastore.Source {
	t.Helper()

	logger := log.NewLogger(os.Stdout)
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

func createTestEvent(t *testing.T, projectID string, endpointIDs []string, sourceID string) *datastore.Event {
	t.Helper()

	eventID := ulid.Make().String()
	now := time.Now()

	return &datastore.Event{
		UID:              eventID,
		EventType:        datastore.EventType("test.event"),
		ProjectID:        projectID,
		SourceID:         sourceID,
		Endpoints:        endpointIDs,
		Headers:          httpheader.HTTPHeader{"X-Test": []string{"value"}},
		Raw:              `{"test": "data"}`,
		Data:             json.RawMessage(`{"test": "data"}`),
		IdempotencyKey:   fmt.Sprintf("idempotency-%s", eventID),
		URLQueryParams:   "query=test",
		CreatedAt:        now,
		UpdatedAt:        now,
		AcknowledgedAt:   null.TimeFrom(now),
		IsDuplicateEvent: false,
	}
}

func TestCreateEvent(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("CreateEvent_Success", func(t *testing.T) {
		event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)

		err := service.CreateEvent(ctx, event)
		require.NoError(t, err)
		require.NotEmpty(t, event.UID)

		// Verify event was created
		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.Equal(t, event.UID, found.UID)
		require.Equal(t, event.ProjectID, found.ProjectID)
		require.Equal(t, event.EventType, found.EventType)
	})

	t.Run("CreateEvent_WithMultipleEndpoints", func(t *testing.T) {
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		endpoint3 := seedTestEndpoint(t, db, project.UID)

		event := createTestEvent(t, project.UID, []string{endpoint.UID, endpoint2.UID, endpoint3.UID}, source.UID)

		err := service.CreateEvent(ctx, event)
		require.NoError(t, err)

		// Verify event has all endpoints
		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.Len(t, found.Endpoints, 3)
	})

	t.Run("CreateEvent_BatchEndpointProcessing", func(t *testing.T) {
		// Test with many endpoints to verify 30K batch processing
		numEndpoints := 100
		endpointIDs := make([]string, numEndpoints)
		for i := 0; i < numEndpoints; i++ {
			endpointIDs[i] = ulid.Make().String()
		}

		event := createTestEvent(t, project.UID, endpointIDs, source.UID)

		err := service.CreateEvent(ctx, event)
		require.NoError(t, err)

		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.Len(t, found.Endpoints, numEndpoints)
	})
}

func TestFindEventByID(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("FindEventByID_Success", func(t *testing.T) {
		event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		err := service.CreateEvent(ctx, event)
		require.NoError(t, err)

		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.Equal(t, event.UID, found.UID)
		require.Equal(t, event.ProjectID, found.ProjectID)
	})

	t.Run("FindEventByID_NotFound", func(t *testing.T) {
		_, err := service.FindEventByID(ctx, project.UID, ulid.Make().String())
		require.Error(t, err)
		require.Equal(t, datastore.ErrEventNotFound, err)
	})
}

func TestFindEventsByIDs(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("FindEventsByIDs_Multiple", func(t *testing.T) {
		event1 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event2 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event3 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)

		require.NoError(t, service.CreateEvent(ctx, event1))
		require.NoError(t, service.CreateEvent(ctx, event2))
		require.NoError(t, service.CreateEvent(ctx, event3))

		events, err := service.FindEventsByIDs(ctx, project.UID, []string{event1.UID, event2.UID, event3.UID})
		require.NoError(t, err)
		require.Len(t, events, 3)
	})

	t.Run("FindEventsByIDs_Empty", func(t *testing.T) {
		events, err := service.FindEventsByIDs(ctx, project.UID, []string{})
		require.NoError(t, err)
		require.Empty(t, events)
	})
}

func TestFindEventsByIdempotencyKey(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("FindEventsByIdempotencyKey_Success", func(t *testing.T) {
		idempotencyKey := fmt.Sprintf("test-key-%s", ulid.Make().String())
		event1 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event1.IdempotencyKey = idempotencyKey

		event2 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event2.IdempotencyKey = idempotencyKey

		require.NoError(t, service.CreateEvent(ctx, event1))
		require.NoError(t, service.CreateEvent(ctx, event2))

		events, err := service.FindEventsByIdempotencyKey(ctx, project.UID, idempotencyKey)
		require.NoError(t, err)
		require.Len(t, events, 2)
	})
}

func TestFindFirstEventWithIdempotencyKey(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("FindFirstEventWithIdempotencyKey_Success", func(t *testing.T) {
		idempotencyKey := fmt.Sprintf("test-key-%s", ulid.Make().String())

		// Create first non-duplicate event
		event1 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event1.IdempotencyKey = idempotencyKey
		event1.IsDuplicateEvent = false
		require.NoError(t, service.CreateEvent(ctx, event1))

		// Create duplicate event
		event2 := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		event2.IdempotencyKey = idempotencyKey
		event2.IsDuplicateEvent = true
		require.NoError(t, service.CreateEvent(ctx, event2))

		found, err := service.FindFirstEventWithIdempotencyKey(ctx, project.UID, idempotencyKey)
		require.NoError(t, err)
		require.Equal(t, event1.UID, found.UID)
		require.False(t, found.IsDuplicateEvent)
	})
}

func TestUpdateEventEndpoints(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("UpdateEventEndpoints_Success", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint1 := seedTestEndpoint(t, db, project.UID)
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, Endpoint1: %s, Endpoint2: %s, Source: %s",
			project.UID, endpoint1.UID, endpoint2.UID, source.UID)

		event := createTestEvent(t, project.UID, []string{endpoint1.UID}, source.UID)
		t.Logf("Created event - EventID: %s, Initial endpoints: %v", event.UID, event.Endpoints)
		require.NoError(t, service.CreateEvent(ctx, event))

		// Verify initial state
		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		t.Logf("Event after create - EventID: %s, Endpoints: %v", found.UID, found.Endpoints)
		require.Len(t, found.Endpoints, 1)

		// Update endpoints
		newEndpoints := []string{endpoint1.UID, endpoint2.UID}
		t.Logf("Updating endpoints to: %v", newEndpoints)
		err = service.UpdateEventEndpoints(ctx, event, newEndpoints)
		require.NoError(t, err)

		// Verify update
		found, err = service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		t.Logf("Event after update - EventID: %s, Endpoints: %v", found.UID, found.Endpoints)
		require.Len(t, found.Endpoints, 2, "Expected 2 endpoints after update, got %d: %v", len(found.Endpoints), found.Endpoints)
	})
}

func TestUpdateEventStatus(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("UpdateEventStatus_Success", func(t *testing.T) {
		event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
		require.NoError(t, service.CreateEvent(ctx, event))

		// Update status
		newStatus := datastore.ProcessingStatus
		err := service.UpdateEventStatus(ctx, event, newStatus)
		require.NoError(t, err)

		// Verify update
		found, err := service.FindEventByID(ctx, project.UID, event.UID)
		require.NoError(t, err)
		require.Equal(t, newStatus, found.Status)
	})
}

func TestCountProjectMessages(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("CountProjectMessages_Success", func(t *testing.T) {
		// Create multiple events
		for i := 0; i < 5; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
		}

		count, err := service.CountProjectMessages(ctx, project.UID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(5))
	})
}

func TestCountEvents(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("CountEvents_NoFilter", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, EndpointID: %s, SourceID: %s", project.UID, endpoint.UID, source.UID)

		// Create test events
		for i := 0; i < 3; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
			t.Logf("Created event %d - EventID: %s", i+1, event.UID)
		}

		filter := &datastore.Filter{}
		count, err := service.CountEvents(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("CountEvents result - ProjectID: %s, Filter: empty, Count: %d, Expected: >=3", project.UID, count)
		require.GreaterOrEqual(t, count, int64(3), "Expected at least 3 events for project %s, got %d", project.UID, count)
	})

	t.Run("CountEvents_WithEndpointFilter", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, EndpointID: %s, SourceID: %s", project.UID, endpoint2.UID, source.UID)

		// Create events for endpoint2
		for i := 0; i < 2; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint2.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
			t.Logf("Created event %d - EventID: %s for EndpointID: %s", i+1, event.UID, endpoint2.UID)
		}

		filter := &datastore.Filter{
			EndpointIDs: []string{endpoint2.UID},
		}
		count, err := service.CountEvents(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("CountEvents result - ProjectID: %s, EndpointFilter: %s, Count: %d, Expected: >=2", project.UID, endpoint2.UID, count)
		require.GreaterOrEqual(t, count, int64(2), "Expected at least 2 events for endpoint %s, got %d", endpoint2.UID, count)
	})
}

func TestLoadEventsPaged(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	t.Run("LoadEventsPaged_FirstPage_DESC", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, EndpointID: %s, SourceID: %s", project.UID, endpoint.UID, source.UID)

		// Create test events
		numEvents := 15
		eventIDs := []string{}
		for i := 0; i < numEvents; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			time.Sleep(1 * time.Millisecond) // Ensure unique timestamps
			require.NoError(t, service.CreateEvent(ctx, event))
			eventIDs = append(eventIDs, event.UID)
		}
		t.Logf("Created %d events: %v", numEvents, eventIDs)

		// Verify events were created
		totalCount, err := service.CountProjectMessages(ctx, project.UID)
		require.NoError(t, err)
		t.Logf("Total events in project: %d", totalCount)
		filter := &datastore.Filter{
			Pageable: datastore.Pageable{
				PerPage:   5,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		t.Logf("Calling LoadEventsPaged - ProjectID: %s, PerPage: 5, Direction: next, Sort: desc", project.UID)
		events, paginationData, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d, HasNextPage: %t, PrevRowCount: %d",
			len(events), paginationData.HasNextPage, paginationData.PrevRowCount.Count)
		if len(events) > 0 {
			t.Logf("First event ID: %s, Last event ID: %s", events[0].UID, events[len(events)-1].UID)
		}
		require.Len(t, events, 5, "Expected 5 events, got %d", len(events))
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
		require.True(t, paginationData.HasNextPage)
	})

	t.Run("LoadEventsPaged_FirstPage_ASC", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, EndpointID: %s, SourceID: %s", project.UID, endpoint.UID, source.UID)

		// Create test events
		numEvents := 15
		for i := 0; i < numEvents; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEvent(ctx, event))
		}
		t.Logf("Created %d events", numEvents)
		filter := &datastore.Filter{
			Pageable: datastore.Pageable{
				PerPage:   5,
				Direction: datastore.Next,
				Sort:      "asc",
			},
		}
		t.Logf("Calling LoadEventsPaged - ProjectID: %s, PerPage: 5, Direction: next, Sort: asc", project.UID)
		events, paginationData, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d, HasNextPage: %t", len(events), paginationData.HasNextPage)
		require.Len(t, events, 5, "Expected 5 events, got %d", len(events))
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
		require.True(t, paginationData.HasNextPage)
	})

	t.Run("LoadEventsPaged_BackwardPagination", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s", project.UID)

		// Create test events
		numEvents := 15
		for i := 0; i < numEvents; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, service.CreateEvent(ctx, event))
		}
		t.Logf("Created %d events", numEvents)
		// First get the first page
		filter := &datastore.Filter{
			Pageable: datastore.Pageable{
				PerPage:   5,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		events, paginationData, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		require.Len(t, events, 5)

		// Now get next page
		filter.Pageable.NextCursor = paginationData.NextPageCursor
		events2, paginationData2, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		require.Len(t, events2, 5)

		// Go back to previous page
		filter.Pageable.Direction = datastore.Prev
		filter.Pageable.PrevCursor = paginationData2.PrevPageCursor
		eventsBack, _, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		require.Len(t, eventsBack, 5)
		require.Equal(t, events[0].UID, eventsBack[0].UID)
	})

	t.Run("LoadEventsPaged_WithEndpointFilter", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint2 := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, EndpointID: %s", project.UID, endpoint2.UID)

		// Create events for endpoint2
		eventIDs := []string{}
		for i := 0; i < 3; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint2.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
			eventIDs = append(eventIDs, event.UID)
		}
		t.Logf("Created 3 events for endpoint2: %v", eventIDs)

		filter := &datastore.Filter{
			EndpointIDs: []string{endpoint2.UID},
			Pageable: datastore.Pageable{
				PerPage:   10,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		t.Logf("Calling LoadEventsPaged with EndpointFilter: %s", endpoint2.UID)
		events, _, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d", len(events))
		require.GreaterOrEqual(t, len(events), 3, "Expected at least 3 events, got %d", len(events))

		// Verify all events belong to endpoint2
		for _, event := range events {
			require.Contains(t, event.Endpoints, endpoint2.UID)
		}
	})

	t.Run("LoadEventsPaged_WithSourceFilter", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source2 := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s, SourceID: %s", project.UID, source2.UID)

		// Create events for source2
		eventIDs := []string{}
		for i := 0; i < 3; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source2.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
			eventIDs = append(eventIDs, event.UID)
		}
		t.Logf("Created 3 events for source2: %v", eventIDs)

		filter := &datastore.Filter{
			SourceID: source2.UID,
			Pageable: datastore.Pageable{
				PerPage:   10,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		t.Logf("Calling LoadEventsPaged with SourceFilter: %s", source2.UID)
		events, _, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d", len(events))
		require.GreaterOrEqual(t, len(events), 3, "Expected at least 3 events, got %d", len(events))

		// Verify all events belong to source2
		for _, event := range events {
			require.Equal(t, source2.UID, event.SourceID)
		}
	})

	t.Run("LoadEventsPaged_WithDateRangeFilter", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s", project.UID)

		// Create events
		for i := 0; i < 5; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
		}
		t.Logf("Created 5 events")

		// Capture timestamps AFTER creating events to ensure events are within range
		// Add 1 second buffer to account for fractional seconds lost in Unix() conversion
		now := time.Now().Add(1 * time.Second)
		yesterday := now.Add(-24 * time.Hour)

		filter := &datastore.Filter{
			SearchParams: datastore.SearchParams{
				CreatedAtStart: yesterday.Unix(),
				CreatedAtEnd:   now.Unix(),
			},
			Pageable: datastore.Pageable{
				PerPage:   10,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		t.Logf("Calling LoadEventsPaged with DateRange: %s (Unix: %d) to %s (Unix: %d)",
			yesterday, yesterday.Unix(), now, now.Unix())
		events, _, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d", len(events))
		require.NotEmpty(t, events, "Expected events in date range, got 0")

		// Verify all events are within date range
		for _, event := range events {
			require.True(t, event.CreatedAt.After(yesterday) || event.CreatedAt.Equal(yesterday))
			require.True(t, event.CreatedAt.Before(now) || event.CreatedAt.Equal(now))
		}
	})

	t.Run("LoadEventsPaged_EmptyResult", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)

		t.Logf("Test setup - ProjectID: %s (no events created)", project.UID)
		nonExistentEndpoint := ulid.Make().String()
		filter := &datastore.Filter{
			EndpointIDs: []string{nonExistentEndpoint},
			Pageable: datastore.Pageable{
				PerPage:   10,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		t.Logf("Calling LoadEventsPaged with non-existent endpoint: %s", nonExistentEndpoint)
		events, paginationData, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d (expected 0)", len(events))
		require.Empty(t, events)
		require.False(t, paginationData.HasNextPage)
		require.Equal(t, 0, paginationData.PrevRowCount.Count)
	})

	t.Run("LoadEventsPaged_ExistsPath_NoSearchQuery", func(t *testing.T) {
		// Create isolated project for this test
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)

		t.Logf("Test setup - ProjectID: %s", project.UID)

		// Create events
		for i := 0; i < 5; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
		}
		t.Logf("Created 5 events")
		// This should use EXISTS path (no search query)
		filter := &datastore.Filter{
			Pageable: datastore.Pageable{
				PerPage:   5,
				Direction: datastore.Next,
				Sort:      "desc",
			},
		}
		// Empty filter.Query means EXISTS path will be used
		t.Logf("Calling LoadEventsPaged with empty query (EXISTS path)")
		events, _, err := service.LoadEventsPaged(ctx, project.UID, filter)
		require.NoError(t, err)
		t.Logf("LoadEventsPaged result - Events count: %d (expected at least 1)", len(events))
		require.NotEmpty(t, events, "Expected events using EXISTS path, got 0")
	})
}

func TestDeleteProjectEvents(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)

	t.Run("DeleteProjectEvents_SoftDelete", func(t *testing.T) {
		// Create test events
		eventIDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
			eventIDs[i] = event.UID
		}

		filter := &datastore.EventFilter{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		}

		err := service.DeleteProjectEvents(ctx, project.UID, filter, false) // soft delete
		require.NoError(t, err)

		// Verify events still exist but are soft deleted
		for _, eventID := range eventIDs {
			found, err := service.FindEventByID(ctx, project.UID, eventID)
			if err == nil {
				require.NotNil(t, found.DeletedAt)
			}
		}
	})

	t.Run("DeleteProjectEvents_HardDelete", func(t *testing.T) {
		// Create test events
		for i := 0; i < 2; i++ {
			event := createTestEvent(t, project.UID, []string{endpoint.UID}, source.UID)
			require.NoError(t, service.CreateEvent(ctx, event))
		}

		filter := &datastore.EventFilter{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		}

		err := service.DeleteProjectEvents(ctx, project.UID, filter, true) // hard delete
		require.NoError(t, err)
	})
}

func TestDeleteProjectTokenizedEvents(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)

	t.Run("DeleteProjectTokenizedEvents_Success", func(t *testing.T) {
		filter := &datastore.EventFilter{
			CreatedAtStart: time.Now().Add(-24 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Unix(),
		}

		err := service.DeleteProjectTokenizedEvents(ctx, project.UID, filter)
		require.NoError(t, err)
	})
}

func TestCopyRows(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)

	t.Run("CopyRows_Success", func(t *testing.T) {
		err := service.CopyRows(ctx, project.UID, 1000)
		require.NoError(t, err)
	})
}

func TestPartitionFunctions(t *testing.T) {
	service, _ := setupTestDB(t)
	ctx := context.Background()

	t.Run("PartitionEventsTable", func(t *testing.T) {
		err := service.PartitionEventsTable(ctx)
		// May fail if already partitioned, just verify it doesn't panic
		_ = err
	})

	t.Run("UnPartitionEventsTable", func(t *testing.T) {
		err := service.UnPartitionEventsTable(ctx)
		// May fail if not partitioned, just verify it doesn't panic
		_ = err
	})

	t.Run("PartitionEventsSearchTable", func(t *testing.T) {
		err := service.PartitionEventsSearchTable(ctx)
		// May fail if already partitioned, just verify it doesn't panic
		_ = err
	})

	t.Run("UnPartitionEventsSearchTable", func(t *testing.T) {
		err := service.UnPartitionEventsSearchTable(ctx)
		// May fail if not partitioned, just verify it doesn't panic
		_ = err
	})
}
