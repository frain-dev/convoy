package event_types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_DeprecateEventType(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create an event type to deprecate
	eventType := seedEventType(t, db, project, "deprecated.event", "To be deprecated", "test", []byte(`{}`))

	before := time.Now().Add(-time.Second)

	// Deprecate the event type
	deprecated, err := service.DeprecateEventType(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.NotNil(t, deprecated)

	after := time.Now().Add(time.Second)

	// Verify the event type was deprecated
	require.Equal(t, eventType.UID, deprecated.UID)
	require.Equal(t, eventType.Name, deprecated.Name)
	require.True(t, deprecated.DeprecatedAt.Valid)
	require.True(t, deprecated.DeprecatedAt.Time.After(before) && deprecated.DeprecatedAt.Time.Before(after))
}

func TestService_DeprecateEventType_ReturnsFullEntity(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create an event type with all fields
	eventType := seedEventType(t, db, project, "full.event", "Full description", "full-category", []byte(`{"full": "schema"}`))

	// Deprecate it
	deprecated, err := service.DeprecateEventType(ctx, eventType.UID, project.UID)
	require.NoError(t, err)

	// Verify all fields are returned
	require.Equal(t, eventType.UID, deprecated.UID)
	require.Equal(t, eventType.Name, deprecated.Name)
	require.Equal(t, eventType.ProjectId, deprecated.ProjectId)
	require.Equal(t, eventType.Description, deprecated.Description)
	require.Equal(t, eventType.Category, deprecated.Category)
	require.Equal(t, eventType.JSONSchema, deprecated.JSONSchema)
	require.False(t, deprecated.CreatedAt.IsZero())
	require.False(t, deprecated.UpdatedAt.IsZero())
	require.True(t, deprecated.DeprecatedAt.Valid)
	require.False(t, deprecated.DeprecatedAt.Time.IsZero())
}

func TestService_DeprecateEventType_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Try to deprecate non-existent event type
	_, err := service.DeprecateEventType(ctx, "non-existent-id", project.UID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestService_DeprecateEventType_VerifyTimestamp(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	eventType := seedEventType(t, db, project, "timestamp.test", "desc", "cat", []byte(`{}`))

	// Ensure event type is not deprecated initially
	fetched, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.False(t, fetched.DeprecatedAt.Valid)

	// Deprecate it
	deprecated, err := service.DeprecateEventType(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.True(t, deprecated.DeprecatedAt.Valid)
	require.False(t, deprecated.DeprecatedAt.Time.IsZero())

	// Verify via fetch
	fetched2, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.True(t, fetched2.DeprecatedAt.Valid)
	require.Equal(t, deprecated.DeprecatedAt.Time, fetched2.DeprecatedAt.Time)
}
