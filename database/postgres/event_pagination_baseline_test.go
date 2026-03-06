//go:build integration
// +build integration

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
)

// Test Case 1: Empty search query uses EXISTS path
func Test_LoadEventsPaged_EmptySearch_UsesExistsPath(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID))

	// Create test events
	for i := 1; i <= 3; i++ {
		event := createTestEvent(project.UID, []string{endpoint.UID}, "", fmt.Sprintf(`{"test": "event %d"}`, i))
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
		time.Sleep(10 * time.Millisecond)
	}

	filter := &datastore.Filter{
		Query:       "", // Empty search - should use EXISTS path
		EndpointIDs: []string{endpoint.UID},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, pagData, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.False(t, pagData.HasNextPage)
}

// Test Case 2: Forward pagination with DESC sort
func Test_LoadEventsPaged_ForwardPagination_DESC(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID))

	// Create 5 events
	for i := 1; i <= 5; i++ {
		event := createTestEvent(project.UID, []string{endpoint.UID}, "", fmt.Sprintf(`{"event": %d}`, i))
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
		time.Sleep(10 * time.Millisecond)
	}

	// First page
	filter := &datastore.Filter{
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   2,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, pagData, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.True(t, pagData.HasNextPage)
	require.NotEmpty(t, pagData.NextPageCursor)

	// Second page
	filter.Pageable.NextCursor = pagData.NextPageCursor
	events, pagData, err = eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.True(t, pagData.HasNextPage)
}

// Test Case 3: Forward pagination with ASC sort
func Test_LoadEventsPaged_ForwardPagination_ASC(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID))

	// Create 5 events
	for i := 1; i <= 5; i++ {
		event := createTestEvent(project.UID, []string{endpoint.UID}, "", fmt.Sprintf(`{"event": %d}`, i))
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
		time.Sleep(10 * time.Millisecond)
	}

	filter := &datastore.Filter{
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   2,
			Direction: datastore.Next,
			Sort:      "ASC",
		},
	}

	events, pagData, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.True(t, pagData.HasNextPage)

	// Verify ASC order (oldest first)
	require.True(t, events[0].CreatedAt.Before(events[1].CreatedAt) ||
		events[0].CreatedAt.Equal(events[1].CreatedAt))
}

// Test Case 4: Filter by endpoint IDs (EXISTS path)
func Test_LoadEventsPaged_FilterByEndpointIDs(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint1 := generateEndpoint(project)
	endpoint2 := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint1, project.UID))
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint2, project.UID))

	// Create events for different endpoints
	event1 := createTestEvent(project.UID, []string{endpoint1.UID}, "", `{"endpoint": "1"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event1))
	time.Sleep(10 * time.Millisecond)

	event2 := createTestEvent(project.UID, []string{endpoint2.UID}, "", `{"endpoint": "2"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event2))

	filter := &datastore.Filter{
		EndpointIDs: []string{endpoint1.UID},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, _, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 1) // Only event for endpoint1
}

// Test Case 5: Filter by source IDs (EXISTS path)
func Test_LoadEventsPaged_FilterBySourceIDs(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID))

	source1 := createTestSource(t, db, project.UID, "source-1")
	source2 := createTestSource(t, db, project.UID, "source-2")

	// Create events from different sources
	event1 := createTestEvent(project.UID, []string{endpoint.UID}, source1, `{"source": "1"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event1))
	time.Sleep(10 * time.Millisecond)

	event2 := createTestEvent(project.UID, []string{endpoint.UID}, source2, `{"source": "2"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event2))

	filter := &datastore.Filter{
		SourceIDs: []string{source1},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, _, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 1) // Only event from source1
	require.Equal(t, source1, events[0].SourceID)
}

// Test Case 6: Filter by owner_id (EXISTS path)
func Test_LoadEventsPaged_FilterByOwnerID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)

	endpoint1 := generateEndpoint(project)
	endpoint1.OwnerID = "owner-1"
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint1, project.UID))

	endpoint2 := generateEndpoint(project)
	endpoint2.OwnerID = "owner-2"
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint2, project.UID))

	// Create events for different owners
	event1 := createTestEvent(project.UID, []string{endpoint1.UID}, "", `{"owner": "1"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event1))
	time.Sleep(10 * time.Millisecond)

	event2 := createTestEvent(project.UID, []string{endpoint2.UID}, "", `{"owner": "2"}`)
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event2))

	filter := &datastore.Filter{
		OwnerID: "owner-1",
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, _, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 1) // Only events for owner-1 endpoints
}

// Test Case 7: Empty result set
func Test_LoadEventsPaged_EmptyResultSet(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)

	filter := &datastore.Filter{
		EndpointIDs: []string{"non-existent-endpoint"},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	events, pagData, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.Len(t, events, 0)
	require.False(t, pagData.HasNextPage)
	require.False(t, pagData.HasPrevPage)
}

// Test Case 8: PrevRowCount calculation (middle page)
func Test_LoadEventsPaged_PrevRowCount_MiddlePage(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	project := seedTestProject(t, db)
	endpoint := generateEndpoint(project)
	require.NoError(t, NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID))

	// Create 10 events
	for i := 1; i <= 10; i++ {
		event := createTestEvent(project.UID, []string{endpoint.UID}, "", fmt.Sprintf(`{"event": %d}`, i))
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
		time.Sleep(10 * time.Millisecond)
	}

	// Get first page
	filter := &datastore.Filter{
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-1 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(1 * time.Hour).Unix(),
		},
		Pageable: datastore.Pageable{
			PerPage:   3,
			Direction: datastore.Next,
			Sort:      "DESC",
		},
	}

	_, pagData, err := eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.True(t, pagData.HasNextPage)

	// Get middle page (should have previous)
	filter.Pageable.NextCursor = pagData.NextPageCursor
	_, pagData, err = eventRepo.LoadEventsPaged(context.Background(), project.UID, filter)
	require.NoError(t, err)
	require.True(t, pagData.HasPrevPage) // Middle page has previous
}

// Helper Functions

func seedTestProject(t *testing.T, db database.Database) *datastore.Project {
	project := &datastore.Project{
		UID:  ulid.Make().String(),
		Name: "test-project-" + ulid.Make().String(),
		Type: "outgoing",
		Config: &datastore.ProjectConfig{
			Strategy: &datastore.StrategyConfiguration{
				Type: "linear",
			},
			Signature: &datastore.SignatureConfiguration{},
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{
				Policy: "72h",
			},
		},
	}
	err := NewProjectRepo(db).CreateProject(context.Background(), project)
	require.NoError(t, err)
	return project
}

func createTestEvent(projectID string, endpoints []string, sourceID string, raw string) *datastore.Event {
	data := json.RawMessage([]byte(raw))

	event := &datastore.Event{
		UID:       ulid.Make().String(),
		EventType: "test.event",
		Endpoints: endpoints,
		ProjectID: projectID,
		SourceID:  sourceID,
		Headers:   httpheader.HTTPHeader{},
		Raw:       raw,
		Data:      data,
	}

	return event
}

func createTestSource(t *testing.T, db database.Database, projectID string, name string) string {
	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      name,
		ProjectID: projectID,
		Type:      "http",
	}
	err := NewSourceRepo(db).CreateSource(context.Background(), source)
	require.NoError(t, err)
	return source.UID
}
