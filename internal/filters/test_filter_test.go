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

func TestTestFilter_DisabledExactFilterFallsBackToCatchAll(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")
	seedEventType(t, db, project.UID, "*")

	service := createFilterService(t, db)

	err := service.CreateFilter(ctx, &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.created",
		EnabledAtSet:   true,
		Headers:        datastore.M{},
		Body:           datastore.M{"priority": "low"},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{"priority": "low"},
	})
	require.NoError(t, err)

	err = service.CreateFilter(ctx, &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "*",
		Headers:        datastore.M{},
		Body:           datastore.M{"priority": "high"},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{"priority": "high"},
	})
	require.NoError(t, err)

	matches, err := service.TestFilter(ctx, subscription.UID, "user.created", map[string]any{"priority": "high"})

	require.NoError(t, err)
	require.True(t, matches)
}

func TestTestFilter_DisabledExactFilterWithoutCatchAllDoesNotMatch(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "user.created")

	service := createFilterService(t, db)

	err := service.CreateFilter(ctx, &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "user.created",
		EnabledAtSet:   true,
		Headers:        datastore.M{},
		Body:           datastore.M{"priority": "low"},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{"priority": "low"},
	})
	require.NoError(t, err)

	matches, err := service.TestFilter(ctx, subscription.UID, "user.created", map[string]any{"priority": "low"})

	require.NoError(t, err)
	require.False(t, matches)
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

func TestTestFilter_MatchesNonBodyScopes(t *testing.T) {
	tests := []struct {
		name    string
		filter  *datastore.EventTypeFilter
		request datastore.FilterTestRequest
		want    bool
	}{
		{
			name: "header only filter matches",
			filter: &datastore.EventTypeFilter{
				Headers:    datastore.M{"X-Event-Type": "push"},
				RawHeaders: datastore.M{"X-Event-Type": "push"},
			},
			request: datastore.FilterTestRequest{
				Headers: datastore.M{"X-Event-Type": "push"},
			},
			want: true,
		},
		{
			name: "query only filter matches",
			filter: &datastore.EventTypeFilter{
				Query:    datastore.M{"event_type": "push"},
				RawQuery: datastore.M{"event_type": "push"},
			},
			request: datastore.FilterTestRequest{
				Query: datastore.M{"event_type": "push"},
			},
			want: true,
		},
		{
			name: "nested query filter matches",
			filter: &datastore.EventTypeFilter{
				Query: datastore.M{
					"metadata": map[string]any{"version": "1.0"},
				},
				RawQuery: datastore.M{
					"metadata": map[string]any{"version": "1.0"},
				},
			},
			request: datastore.FilterTestRequest{
				Query: datastore.M{
					"metadata": map[string]any{"version": "1.0"},
				},
			},
			want: true,
		},
		{
			name: "path only filter matches",
			filter: &datastore.EventTypeFilter{
				Path:    datastore.M{"path": "/ingest/source-id"},
				RawPath: datastore.M{"path": "/ingest/source-id"},
			},
			request: datastore.FilterTestRequest{
				Path: datastore.M{"path": "/ingest/source-id"},
			},
			want: true,
		},
		{
			name: "multi scope filter fails when one scope misses",
			filter: &datastore.EventTypeFilter{
				Body: datastore.M{
					"event": "push",
				},
				Query:    datastore.M{"ref": "main"},
				RawBody:  datastore.M{"event": "push"},
				RawQuery: datastore.M{"ref": "main"},
			},
			request: datastore.FilterTestRequest{
				Body:  datastore.M{"event": "push"},
				Query: datastore.M{"ref": "develop"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, ctx := setupTestDB(t)
			project, subscription := seedTestData(t, db)
			seedEventType(t, db, project.UID, "test.event")

			service := createFilterService(t, db)
			tt.filter.SubscriptionID = subscription.UID
			tt.filter.EventType = "test.event"
			if tt.filter.Headers == nil {
				tt.filter.Headers = datastore.M{}
			}
			if tt.filter.Body == nil {
				tt.filter.Body = datastore.M{}
			}
			if tt.filter.Query == nil {
				tt.filter.Query = datastore.M{}
			}
			if tt.filter.Path == nil {
				tt.filter.Path = datastore.M{}
			}
			if tt.filter.RawHeaders == nil {
				tt.filter.RawHeaders = datastore.M{}
			}
			if tt.filter.RawBody == nil {
				tt.filter.RawBody = datastore.M{}
			}
			if tt.filter.RawQuery == nil {
				tt.filter.RawQuery = datastore.M{}
			}
			if tt.filter.RawPath == nil {
				tt.filter.RawPath = datastore.M{}
			}

			err := service.CreateFilter(ctx, tt.filter)
			require.NoError(t, err)

			matches, err := service.TestFilter(ctx, subscription.UID, "test.event", tt.request)

			require.NoError(t, err)
			require.Equal(t, tt.want, matches)
		})
	}
}

func TestPrepareFilterMapsRejectsArrayWildcardSelectorsInNonBodyScopes(t *testing.T) {
	tests := []struct {
		name   string
		filter *datastore.EventTypeFilter
		scope  string
	}{
		{
			name:  "headers",
			scope: "header",
			filter: &datastore.EventTypeFilter{
				Headers: datastore.M{"items": datastore.M{"$": datastore.M{"id": "123"}}},
			},
		},
		{
			name:  "query",
			scope: "query",
			filter: &datastore.EventTypeFilter{
				Query: datastore.M{"items.$.id": "123"},
			},
		},
		{
			name:  "path",
			scope: "path",
			filter: &datastore.EventTypeFilter{
				Path: datastore.M{"items.$.id": "123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prepareFilterMaps(tt.filter)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.scope)
		})
	}
}
