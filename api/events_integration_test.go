package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/httpheader"
)

type EventsIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	ConvoyApp      *ApplicationHandler
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
}

func (s *EventsIntegrationTestSuite) SetupSuite() {
	s.ConvoyApp = buildServer(s.T())
}

func (s *EventsIntegrationTestSuite) SetupTest() {
	var err error

	s.DB = s.ConvoyApp.A.DB

	// Seed default user
	s.DefaultUser, err = testdb.SeedDefaultUser(s.DB)
	require.NoError(s.T(), err)

	// Seed default organisation
	org, err := testdb.SeedDefaultOrganisation(s.DB, s.DefaultUser)
	require.NoError(s.T(), err)

	// Seed default project
	s.DefaultProject, err = testdb.SeedDefaultProject(s.DB, org.UID)
	require.NoError(s.T(), err)
}

func (s *EventsIntegrationTestSuite) TearDownTest() {
	metrics.Reset()
}

// Test_LoadEventsPaged_WithoutEndpoints tests that events without endpoint associations
// are visible in the event log when no endpoint filter is applied.
func (s *EventsIntegrationTestSuite) Test_LoadEventsPaged_WithoutEndpoints() {
	ctx := context.Background()
	eventRepo := postgres.NewEventRepo(s.DB)

	data := json.RawMessage([]byte(`{"test": "data"}`))

	// Create an event with no endpoints (simulating an event ingested for a source with no subscriptions)
	event := &datastore.Event{
		UID:       ulid.Make().String(),
		EventType: "test-event-no-endpoints",
		Endpoints: []string{}, // Empty endpoints array
		ProjectID: s.DefaultProject.UID,
		Headers:   httpheader.HTTPHeader{},
		Raw:       string(data),
		Data:      data,
		Status:    datastore.FailureStatus, // Events without subscriptions get Failure status
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := eventRepo.CreateEvent(ctx, event)
	require.NoError(s.T(), err)

	// Query without endpoint filter - should return the event
	events, _, err := eventRepo.LoadEventsPaged(ctx, s.DefaultProject.UID, &datastore.Filter{
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(5 * time.Minute).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:    10,
			Direction:  datastore.Next,
			NextCursor: datastore.DefaultCursor,
		},
	})

	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, len(events))
	require.Equal(s.T(), event.UID, events[0].UID)
	require.Equal(s.T(), datastore.FailureStatus, events[0].Status)
}

// Test_LoadEventsPaged_WithEndpointFilter tests that:
// 1. When filtering by endpoint, only events with that endpoint are returned
// 2. When not filtering, both events with and without endpoints are returned
func (s *EventsIntegrationTestSuite) Test_LoadEventsPaged_WithEndpointFilter() {
	ctx := context.Background()
	eventRepo := postgres.NewEventRepo(s.DB)

	// Create an endpoint
	endpoint, err := testdb.SeedEndpoint(s.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	data := json.RawMessage([]byte(`{"test": "data"}`))

	// Create event with endpoint
	eventWithEndpoint := &datastore.Event{
		UID:       ulid.Make().String(),
		EventType: "test-event-with-endpoint",
		Endpoints: []string{endpoint.UID},
		ProjectID: s.DefaultProject.UID,
		Headers:   httpheader.HTTPHeader{},
		Raw:       string(data),
		Data:      data,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = eventRepo.CreateEvent(ctx, eventWithEndpoint)
	require.NoError(s.T(), err)

	// Create event without endpoint
	eventWithoutEndpoint := &datastore.Event{
		UID:       ulid.Make().String(),
		EventType: "test-event-without-endpoint",
		Endpoints: []string{},
		ProjectID: s.DefaultProject.UID,
		Headers:   httpheader.HTTPHeader{},
		Raw:       string(data),
		Data:      data,
		Status:    datastore.FailureStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = eventRepo.CreateEvent(ctx, eventWithoutEndpoint)
	require.NoError(s.T(), err)

	// Query with endpoint filter - should only return event with matching endpoint
	events, _, err := eventRepo.LoadEventsPaged(ctx, s.DefaultProject.UID, &datastore.Filter{
		EndpointID: endpoint.UID,
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(5 * time.Minute).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:    10,
			Direction:  datastore.Next,
			NextCursor: datastore.DefaultCursor,
		},
	})

	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, len(events))
	require.Equal(s.T(), eventWithEndpoint.UID, events[0].UID)

	// Query without endpoint filter - should return both events
	events, _, err = eventRepo.LoadEventsPaged(ctx, s.DefaultProject.UID, &datastore.Filter{
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(5 * time.Minute).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:    10,
			Direction:  datastore.Next,
			NextCursor: datastore.DefaultCursor,
		},
	})

	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(events))
}

// Test_LoadEventsPaged_SearchWithoutEndpoints tests that events without endpoints
// are included in search results
func (s *EventsIntegrationTestSuite) Test_LoadEventsPaged_SearchWithoutEndpoints() {
	ctx := context.Background()
	eventRepo := postgres.NewEventRepo(s.DB)

	data := json.RawMessage([]byte(`{"unique_search_term": "test12345"}`))

	// Create an event with no endpoints but searchable content
	event := &datastore.Event{
		UID:       ulid.Make().String(),
		EventType: "test-event-searchable",
		Endpoints: []string{},
		ProjectID: s.DefaultProject.UID,
		Headers:   httpheader.HTTPHeader{},
		Raw:       string(data),
		Data:      data,
		Status:    datastore.FailureStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := eventRepo.CreateEvent(ctx, event)
	require.NoError(s.T(), err)

	// Copy to search table for text search
	err = eventRepo.CopyRows(ctx, s.DefaultProject.UID, 1)
	require.NoError(s.T(), err)

	// Search for the event - should find it despite no endpoints
	events, _, err := eventRepo.LoadEventsPaged(ctx, s.DefaultProject.UID, &datastore.Filter{
		Query: "unique_search_term",
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(5 * time.Minute).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:    10,
			Direction:  datastore.Next,
			NextCursor: datastore.DefaultCursor,
		},
	})

	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, len(events))
	require.Equal(s.T(), event.UID, events[0].UID)
}

func TestEventsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EventsIntegrationTestSuite))
}
