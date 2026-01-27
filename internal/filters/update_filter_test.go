package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// TestUpdateFilter_Success tests updating a filter
func TestUpdateFilter_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "test.event")
	seedEventType(t, db, project.UID, "updated.event")

	service := createFilterService(t, db)

	// Create a filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "test.event",
		Headers:        datastore.M{"old": "header"},
		Body:           datastore.M{"old": "body"},
		RawHeaders:     datastore.M{"old": "header"},
		RawBody:        datastore.M{"old": "body"},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Update the filter
	filter.EventType = "updated.event"
	filter.Headers = datastore.M{"new": "header"}
	filter.Body = datastore.M{"new": "body"}
	filter.RawHeaders = datastore.M{"new": "header"}
	filter.RawBody = datastore.M{"new": "body"}

	err = service.UpdateFilter(ctx, filter)

	require.NoError(t, err)

	// Verify update
	updated, err := service.FindFilterByID(ctx, filter.UID)
	require.NoError(t, err)
	require.Equal(t, "updated.event", updated.EventType)
	require.NotNil(t, updated.Headers)
	require.NotNil(t, updated.Body)
}

// TestUpdateFilter_NotFound tests updating non-existent filter
func TestUpdateFilter_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	filter := &datastore.EventTypeFilter{
		UID:            "non-existent",
		SubscriptionID: "sub-id",
		EventType:      "event",
		Headers:        datastore.M{},
		Body:           datastore.M{},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{},
	}

	err := service.UpdateFilter(ctx, filter)

	require.Error(t, err)
	require.Equal(t, datastore.ErrFilterNotFound, err)
}

// TestUpdateFilter_NilFilter tests updating with nil filter
func TestUpdateFilter_NilFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	err := service.UpdateFilter(ctx, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "filter cannot be nil")
}

// TestUpdateFilters_BulkUpdate tests updating multiple filters
func TestUpdateFilters_BulkUpdate(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "event.1")
	seedEventType(t, db, project.UID, "event.2")

	service := createFilterService(t, db)

	// Create filters
	filters := []datastore.EventTypeFilter{
		{
			SubscriptionID: subscription.UID,
			EventType:      "event.1",
			Headers:        datastore.M{"version": "1"},
			Body:           datastore.M{"status": "old"},
			RawHeaders:     datastore.M{"version": "1"},
			RawBody:        datastore.M{"status": "old"},
		},
		{
			SubscriptionID: subscription.UID,
			EventType:      "event.2",
			Headers:        datastore.M{"version": "1"},
			Body:           datastore.M{"status": "old"},
			RawHeaders:     datastore.M{"version": "1"},
			RawBody:        datastore.M{"status": "old"},
		},
	}
	err := service.CreateFilters(ctx, filters)
	require.NoError(t, err)

	// Update filters
	for i := range filters {
		filters[i].Headers = datastore.M{"version": "2"}
		filters[i].Body = datastore.M{"status": "new"}
		filters[i].RawHeaders = datastore.M{"version": "2"}
		filters[i].RawBody = datastore.M{"status": "new"}
	}

	err = service.UpdateFilters(ctx, filters)

	require.NoError(t, err)

	// Verify updates
	for _, filter := range filters {
		updated, err := service.FindFilterByID(ctx, filter.UID)
		require.NoError(t, err)
		require.NotNil(t, updated.Headers)
		require.NotNil(t, updated.Body)
	}
}
