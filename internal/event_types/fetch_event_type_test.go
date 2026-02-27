package event_types

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestService_FetchEventTypeById(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create event type
	eventType := seedEventType(t, db, project, "fetch.by.id", "Test description", "test-category", []byte(`{"test": "schema"}`))

	// Fetch it
	fetched, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, eventType.UID, fetched.UID)
	require.Equal(t, eventType.Name, fetched.Name)
	require.Equal(t, eventType.ProjectId, fetched.ProjectId)
	require.Equal(t, eventType.Description, fetched.Description)
	require.Equal(t, eventType.Category, fetched.Category)
	require.Equal(t, eventType.JSONSchema, fetched.JSONSchema)
}

func TestService_FetchEventTypeById_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Try to fetch non-existent event type
	_, err := service.FetchEventTypeById(ctx, "non-existent-id", project.UID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestService_FetchEventTypeByName(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create event type
	eventType := seedEventType(t, db, project, "fetch.by.name", "Test description", "test-category", []byte(`{"test": "schema"}`))

	// Fetch it by name
	fetched, err := service.FetchEventTypeByName(ctx, "fetch.by.name", project.UID)
	require.NoError(t, err)
	require.Equal(t, eventType.UID, fetched.UID)
	require.Equal(t, eventType.Name, fetched.Name)
	require.Equal(t, eventType.ProjectId, fetched.ProjectId)
}

func TestService_FetchEventTypeByName_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Try to fetch non-existent event type
	_, err := service.FetchEventTypeByName(ctx, "non.existent.name", project.UID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestService_FetchAllEventTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create multiple event types
	et1 := seedEventType(t, db, project, "event.one", "First", "cat1", []byte(`{}`))
	et2 := seedEventType(t, db, project, "event.two", "Second", "cat2", []byte(`{}`))
	et3 := seedEventType(t, db, project, "event.three", "Third", "cat3", []byte(`{}`))

	// Fetch all
	eventTypes, err := service.FetchAllEventTypes(ctx, project.UID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(eventTypes), 3)

	// Verify the event types we created are in the list
	names := make(map[string]bool)
	for _, et := range eventTypes {
		names[et.Name] = true
	}
	require.True(t, names[et1.Name])
	require.True(t, names[et2.Name])
	require.True(t, names[et3.Name])
}

func TestService_FetchAllEventTypes_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Fetch all from empty project
	eventTypes, err := service.FetchAllEventTypes(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, eventTypes)
	require.Empty(t, eventTypes)
}

func TestService_FetchAllEventTypes_OrderedByCreatedAt(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create event types in sequence
	et1 := seedEventType(t, db, project, "event.first", "First", "cat1", []byte(`{}`))
	et2 := seedEventType(t, db, project, "event.second", "Second", "cat2", []byte(`{}`))
	et3 := seedEventType(t, db, project, "event.third", "Third", "cat3", []byte(`{}`))

	// Fetch all
	eventTypes, err := service.FetchAllEventTypes(ctx, project.UID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(eventTypes), 3)

	// Find our event types in the result
	var foundEt1, foundEt2, foundEt3 int
	for i, et := range eventTypes {
		if et.UID == et1.UID {
			foundEt1 = i
		}
		if et.UID == et2.UID {
			foundEt2 = i
		}
		if et.UID == et3.UID {
			foundEt3 = i
		}
	}

	// Verify they are ordered by created_at DESC (newest first)
	require.True(t, foundEt3 < foundEt2, "et3 should come before et2")
	require.True(t, foundEt2 < foundEt1, "et2 should come before et1")
}

func TestService_FetchEventType_NullableFieldsHandled(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create event type with no description or category (NULL)
	eventType := seedEventType(t, db, project, "nullable.test", "", "", []byte(`{}`))

	// Fetch it
	fetched, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.Empty(t, fetched.Description)
	require.Empty(t, fetched.Category)
	require.False(t, fetched.DeprecatedAt.Valid)

	// Create event type with description and category
	eventType2 := seedEventType(t, db, project, "with.fields", "Has description", "has-category", []byte(`{}`))

	// Fetch it
	fetched2, err := service.FetchEventTypeById(ctx, eventType2.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, "Has description", fetched2.Description)
	require.Equal(t, "has-category", fetched2.Category)
}

func TestService_FetchEventTypes_DifferentProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, org, project1 := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create a second project
	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project 2",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &datastore.DefaultProjectConfig,
	}
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	err := projectRepo.CreateProject(ctx, project2)
	require.NoError(t, err)

	// Create event types in different projects
	et1 := seedEventType(t, db, project1, "project1.event", "P1", "cat1", []byte(`{}`))
	et2 := seedEventType(t, db, project2, "project2.event", "P2", "cat2", []byte(`{}`))

	// Fetch from project1
	eventTypes1, err := service.FetchAllEventTypes(ctx, project1.UID)
	require.NoError(t, err)

	// Verify only project1 event types are returned
	for _, et := range eventTypes1 {
		require.Equal(t, project1.UID, et.ProjectId)
		if et.UID == et1.UID {
			require.Equal(t, "project1.event", et.Name)
		}
	}

	// Fetch from project2
	eventTypes2, err := service.FetchAllEventTypes(ctx, project2.UID)
	require.NoError(t, err)

	// Verify only project2 event types are returned
	for _, et := range eventTypes2 {
		require.Equal(t, project2.UID, et.ProjectId)
		if et.UID == et2.UID {
			require.Equal(t, "project2.event", et.Name)
		}
	}
}
