package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// TestFindFilterByID_Found tests finding an existing filter
func TestFindFilterByID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "test.event")

	service := createFilterService(t, db)

	// Create a filter first
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "test.event",
		Headers:        datastore.M{"X-Test": "value"},
		Body:           datastore.M{"field": "value"},
		RawHeaders:     datastore.M{"X-Test": "value"},
		RawBody:        datastore.M{"field": "value"},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Find the filter
	found, err := service.FindFilterByID(ctx, filter.UID)

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, filter.UID, found.UID)
	require.Equal(t, filter.SubscriptionID, found.SubscriptionID)
	require.Equal(t, filter.EventType, found.EventType)
	require.NotNil(t, found.Headers)
	require.NotNil(t, found.Body)
}

// TestFindFilterByID_NotFound tests finding a non-existent filter
func TestFindFilterByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	found, err := service.FindFilterByID(ctx, "non-existent-id")

	require.Error(t, err)
	require.Nil(t, found)
	require.Equal(t, datastore.ErrFilterNotFound, err)
}

// TestFindFiltersBySubscriptionID_Found tests finding filters for a subscription
func TestFindFiltersBySubscriptionID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "event.1")
	seedEventType(t, db, project.UID, "event.2")

	service := createFilterService(t, db)

	// Create multiple filters
	filters := []datastore.EventTypeFilter{
		{
			SubscriptionID: subscription.UID,
			EventType:      "event.1",
			Headers:        datastore.M{},
			Body:           datastore.M{},
			RawHeaders:     datastore.M{},
			RawBody:        datastore.M{},
		},
		{
			SubscriptionID: subscription.UID,
			EventType:      "event.2",
			Headers:        datastore.M{},
			Body:           datastore.M{},
			RawHeaders:     datastore.M{},
			RawBody:        datastore.M{},
		},
	}
	err := service.CreateFilters(ctx, filters)
	require.NoError(t, err)

	// Find filters
	found, err := service.FindFiltersBySubscriptionID(ctx, subscription.UID)

	require.NoError(t, err)
	require.Len(t, found, 2)
}

// TestFindFiltersBySubscriptionID_Empty tests finding filters for subscription with no filters
func TestFindFiltersBySubscriptionID_Empty(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, subscription := seedTestData(t, db)

	service := createFilterService(t, db)

	found, err := service.FindFiltersBySubscriptionID(ctx, subscription.UID)

	require.NoError(t, err)
	require.Empty(t, found)
}

// TestFindFilterBySubscriptionAndEventType_Found tests finding specific filter
func TestFindFilterBySubscriptionAndEventType_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.login")

	service := createFilterService(t, db)

	// Create a filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.login",
		Headers:        datastore.M{},
		Body:           datastore.M{"action": "login"},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{"action": "login"},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Find the filter
	found, err := service.FindFilterBySubscriptionAndEventType(ctx, subscription.UID, "user.login")

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, filter.UID, found.UID)
	require.Equal(t, "user.login", found.EventType)
}

// TestFindFilterBySubscriptionAndEventType_NotFound tests finding non-existent filter
func TestFindFilterBySubscriptionAndEventType_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, subscription := seedTestData(t, db)

	service := createFilterService(t, db)

	found, err := service.FindFilterBySubscriptionAndEventType(ctx, subscription.UID, "non.existent")

	require.Error(t, err)
	require.Nil(t, found)
	require.Equal(t, datastore.ErrFilterNotFound, err)
}
