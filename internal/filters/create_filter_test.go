package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// TestCreateFilter_ValidRequest tests creating a filter with valid data
func TestCreateFilter_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")

	service := createFilterService(t, db)

	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.created",
		Headers: datastore.M{
			"X-Company-ID": "123",
		},
		Body: datastore.M{
			"event": "user.created",
			"data": map[string]any{
				"user_id": "456",
			},
		},
		RawHeaders: datastore.M{
			"X-Company-ID": "123",
		},
		RawBody: datastore.M{
			"event": "user.created",
			"data": map[string]any{
				"user_id": "456",
			},
		},
	}

	err := service.CreateFilter(ctx, filter)

	require.NoError(t, err)
	require.NotEmpty(t, filter.UID)
	require.NotZero(t, filter.CreatedAt)
	require.NotZero(t, filter.UpdatedAt)

	// Verify filter was created
	created, err := service.FindFilterByID(ctx, filter.UID)
	require.NoError(t, err)
	require.Equal(t, filter.UID, created.UID)
	require.Equal(t, subscription.UID, created.SubscriptionID)
	require.Equal(t, "user.created", created.EventType)
	require.NotNil(t, created.Headers)
	require.NotNil(t, created.Body)
}

// TestCreateFilter_WithNilMaps tests creating a filter with nil maps
func TestCreateFilter_WithNilMaps(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "*")

	service := createFilterService(t, db)

	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "*",
		Headers:        nil,
		Body:           nil,
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{},
	}

	err := service.CreateFilter(ctx, filter)

	require.NoError(t, err)
	require.NotEmpty(t, filter.UID)

	// Verify filter was created with empty maps
	created, err := service.FindFilterByID(ctx, filter.UID)
	require.NoError(t, err)
	require.NotNil(t, created.Headers)
	require.NotNil(t, created.Body)
	require.Empty(t, created.Headers)
	require.Empty(t, created.Body)
}

// TestCreateFilter_WithNestedBody tests creating a filter with nested body structure
func TestCreateFilter_WithNestedBody(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "order.created")

	service := createFilterService(t, db)

	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "order.created",
		Headers:        datastore.M{},
		Body: datastore.M{
			"order": map[string]any{
				"status": "pending",
				"items": []any{
					map[string]any{"id": "1", "name": "Item 1"},
					map[string]any{"id": "2", "name": "Item 2"},
				},
			},
		},
		RawHeaders: datastore.M{},
		RawBody: datastore.M{
			"order": map[string]any{
				"status": "pending",
				"items": []any{
					map[string]any{"id": "1", "name": "Item 1"},
					map[string]any{"id": "2", "name": "Item 2"},
				},
			},
		},
	}

	err := service.CreateFilter(ctx, filter)

	require.NoError(t, err)
	require.NotEmpty(t, filter.UID)

	// Verify the body was flattened correctly
	created, err := service.FindFilterByID(ctx, filter.UID)
	require.NoError(t, err)
	require.NotNil(t, created.Body)
	// The flattened body should have dot-notation keys
	require.NotEmpty(t, created.Body)
}

// TestCreateFilter_NilFilter tests creating a nil filter
func TestCreateFilter_NilFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	err := service.CreateFilter(ctx, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "filter cannot be nil")
}

// TestCreateFilters_BulkCreate tests creating multiple filters
func TestCreateFilters_BulkCreate(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")
	seedEventType(t, db, project.UID, "user.updated")
	seedEventType(t, db, project.UID, "user.deleted")

	service := createFilterService(t, db)

	filters := []datastore.EventTypeFilter{
		{
			SubscriptionID: subscription.UID,
			EventType:      "user.created",
			Headers:        datastore.M{},
			Body:           datastore.M{"event": "user.created"},
			RawHeaders:     datastore.M{},
			RawBody:        datastore.M{"event": "user.created"},
		},
		{
			SubscriptionID: subscription.UID,
			EventType:      "user.updated",
			Headers:        datastore.M{},
			Body:           datastore.M{"event": "user.updated"},
			RawHeaders:     datastore.M{},
			RawBody:        datastore.M{"event": "user.updated"},
		},
		{
			SubscriptionID: subscription.UID,
			EventType:      "user.deleted",
			Headers:        datastore.M{},
			Body:           datastore.M{"event": "user.deleted"},
			RawHeaders:     datastore.M{},
			RawBody:        datastore.M{"event": "user.deleted"},
		},
	}

	err := service.CreateFilters(ctx, filters)

	require.NoError(t, err)
	// Verify all filters have UIDs
	for i := range filters {
		require.NotEmpty(t, filters[i].UID)
		require.NotZero(t, filters[i].CreatedAt)
	}

	// Verify all filters were created
	created, err := service.FindFiltersBySubscriptionID(ctx, subscription.UID)
	require.NoError(t, err)
	require.Len(t, created, 3)
}

// TestCreateFilters_EmptyArray tests creating with empty array
func TestCreateFilters_EmptyArray(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	err := service.CreateFilters(ctx, []datastore.EventTypeFilter{})

	require.NoError(t, err) // Should be no-op
}
