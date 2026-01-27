package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// TestTestFilter_MatchesBody tests filter matching with body
func TestTestFilter_MatchesBody(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")

	service := createFilterService(t, db)

	// Create a filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.created",
		Headers:        datastore.M{},
		Body: datastore.M{
			"user": map[string]any{
				"status": "active",
			},
		},
		RawHeaders: datastore.M{},
		RawBody: datastore.M{
			"user": map[string]any{
				"status": "active",
			},
		},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Test matching payload
	payload := map[string]any{
		"user": map[string]any{
			"id":     "123",
			"status": "active",
			"name":   "Test User",
		},
	}

	matches, err := service.TestFilter(ctx, subscription.UID, "user.created", payload)

	require.NoError(t, err)
	require.True(t, matches)
}

// TestTestFilter_DoesNotMatch tests filter that doesn't match
func TestTestFilter_DoesNotMatch(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")

	service := createFilterService(t, db)

	// Create a filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.created",
		Headers:        datastore.M{},
		Body: datastore.M{
			"user": map[string]any{
				"status": "active",
			},
		},
		RawHeaders: datastore.M{},
		RawBody: datastore.M{
			"user": map[string]any{
				"status": "active",
			},
		},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Test non-matching payload
	payload := map[string]any{
		"user": map[string]any{
			"id":     "123",
			"status": "inactive", // Different status
			"name":   "Test User",
		},
	}

	matches, err := service.TestFilter(ctx, subscription.UID, "user.created", payload)

	require.NoError(t, err)
	require.False(t, matches)
}

// TestTestFilter_NoFilter tests when no filter exists (should match)
func TestTestFilter_NoFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, subscription := seedTestData(t, db)

	service := createFilterService(t, db)

	payload := map[string]any{
		"event": "test",
	}

	matches, err := service.TestFilter(ctx, subscription.UID, "non.existent", payload)

	require.NoError(t, err)
	require.True(t, matches) // No filter means it matches
}

// TestTestFilter_CatchAllFilter tests catch-all filter with "*"
func TestTestFilter_CatchAllFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "*")

	service := createFilterService(t, db)

	// Create a catch-all filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "*",
		Headers:        datastore.M{},
		Body: datastore.M{
			"priority": "high",
		},
		RawHeaders: datastore.M{},
		RawBody: datastore.M{
			"priority": "high",
		},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Test with different event type
	payload := map[string]any{
		"priority": "high",
		"message":  "test",
	}

	matches, err := service.TestFilter(ctx, subscription.UID, "any.event.type", payload)

	require.NoError(t, err)
	require.True(t, matches) // Catch-all filter should match
}

// TestTestFilter_EmptyBodyFilter tests filter with empty body (matches everything)
func TestTestFilter_EmptyBodyFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "test.event")

	service := createFilterService(t, db)

	// Create filter with empty body
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "test.event",
		Headers:        datastore.M{},
		Body:           datastore.M{},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	payload := map[string]any{
		"any": "data",
	}

	matches, err := service.TestFilter(ctx, subscription.UID, "test.event", payload)

	require.NoError(t, err)
	require.True(t, matches) // Empty filter matches everything
}
