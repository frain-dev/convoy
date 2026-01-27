package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestCountEndpointSubscriptions(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_count_multiple_subscriptions", func(t *testing.T) {
		// Create 5 subscriptions for the endpoint
		for i := 0; i < 5; i++ {
			sub := createTestSubscription(project, endpoint, source)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, project.UID, sub)
			require.NoError(t, err)
		}

		count, err := service.CountEndpointSubscriptions(ctx, project.UID, endpoint.UID, "")
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, int64(5))
	})

	t.Run("should_exclude_specific_subscription", func(t *testing.T) {
		// Create subscriptions
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		// Count excluding sub1
		count, err := service.CountEndpointSubscriptions(ctx, project.UID, endpoint.UID, sub1.UID)
		require.NoError(t, err)

		// Count including all
		countAll, err := service.CountEndpointSubscriptions(ctx, project.UID, endpoint.UID, "")
		require.NoError(t, err)

		// Count should be 1 less when excluding
		require.Equal(t, countAll-1, count)
	})

	t.Run("should_return_zero_count", func(t *testing.T) {
		// Create a new endpoint with no subscriptions
		newEndpoint := seedEndpoint(t, db, project)

		count, err := service.CountEndpointSubscriptions(ctx, project.UID, newEndpoint.UID, "")
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
	})

	t.Run("should_respect_project_isolation", func(t *testing.T) {
		// Create subscription in first project
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		// Create another project
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		project2 := seedProject(t, db, org)

		// Count subscriptions for endpoint in project2 (should be 0)
		count, err := service.CountEndpointSubscriptions(ctx, project2.UID, endpoint.UID, "")
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
	})
}

func TestTestSubscriptionFilter(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	_, _, _, _ = seedTestData(t, db)

	t.Run("should_match_filter_headers", func(t *testing.T) {
		payload := map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Company-ID": "123",
				"X-API-Key":    "secret",
			},
			"body": map[string]interface{}{
				"event": "user.created",
			},
		}

		filter := map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Company-ID": "123",
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_match_filter_body", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"event":  "user.created",
				"status": "active",
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"event": "user.created",
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_match_flattened_filters", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
		}

		// When isFlattened=true, the filter should be fully flattened
		filter := map[string]interface{}{
			"body.user.name": "John",
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, true)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_not_match", func(t *testing.T) {
		payload := map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Company-ID": "123",
			},
			"body": map[string]interface{}{
				"event": "user.created",
			},
		}

		filter := map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Company-ID": "456", // Different value
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.False(t, matches)
	})

	t.Run("should_match_with_nil_filter", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"event": "user.created",
			},
		}

		// Nil filter should always match
		matches, err := service.TestSubscriptionFilter(ctx, payload, nil, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_match_with_empty_filter", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"event": "user.created",
			},
		}

		// Empty filter should always match
		filter := map[string]interface{}{}
		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_match_complex_nested_structure", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"contact": map[string]interface{}{
							"email": "test@example.com",
						},
					},
					"status": "active",
				},
				"event": "user.created",
			},
		}

		// When isFlattened=true, the filter should be fully flattened
		filter := map[string]interface{}{
			"body.user.status": "active",
			"body.event":       "user.created",
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, true)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_not_match_partial_value", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"status": "inactive",
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"status": "active",
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.False(t, matches)
	})
}

func TestTestSubscriptionFilter_EdgeCases(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	_, _, _, _ = seedTestData(t, db)

	t.Run("should_handle_array_values", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"tags": []string{"important", "urgent"},
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"tags": []string{"important", "urgent"},
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_handle_numeric_values", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"age":   30,
				"score": 95.5,
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"age": 30,
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_handle_boolean_values", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"is_active":  true,
				"is_deleted": false,
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"is_active": true,
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_handle_null_values", func(t *testing.T) {
		payload := map[string]interface{}{
			"body": map[string]interface{}{
				"optional_field": nil,
				"event":          "test.event",
			},
		}

		filter := map[string]interface{}{
			"body": map[string]interface{}{
				"event": "test.event",
			},
		}

		matches, err := service.TestSubscriptionFilter(ctx, payload, filter, false)
		require.NoError(t, err)
		require.True(t, matches)
	})
}

func TestCompareFlattenedPayload(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	_, _, _, _ = seedTestData(t, db)

	t.Run("should_match_flattened_payload", func(t *testing.T) {
		flattenedPayload := map[string]interface{}{
			"user.name":    "John",
			"user.email":   "john@example.com",
			"user.profile": "active",
		}

		flattenedFilter := map[string]interface{}{
			"user.name": "John",
		}

		matches, err := service.TestSubscriptionFilter(ctx, flattenedPayload, flattenedFilter, true)
		require.NoError(t, err)
		require.True(t, matches)
	})

	t.Run("should_not_match_different_values", func(t *testing.T) {
		flattenedPayload := map[string]interface{}{
			"user.name":  "John",
			"user.email": "john@example.com",
		}

		flattenedFilter := map[string]interface{}{
			"user.name": "Jane",
		}

		matches, err := service.TestSubscriptionFilter(ctx, flattenedPayload, flattenedFilter, true)
		require.NoError(t, err)
		require.False(t, matches)
	})

	t.Run("should_handle_nil_payload", func(t *testing.T) {
		flattenedFilter := map[string]interface{}{
			"user.name": "John",
		}

		// Nil payload should not match non-empty filter
		matches, err := service.TestSubscriptionFilter(ctx, nil, flattenedFilter, true)
		require.NoError(t, err)
		require.False(t, matches)
	})

	t.Run("should_handle_nil_filter", func(t *testing.T) {
		flattenedPayload := map[string]interface{}{
			"user.name": "John",
		}

		// Nil filter should always match
		matches, err := service.TestSubscriptionFilter(ctx, flattenedPayload, nil, true)
		require.NoError(t, err)
		require.True(t, matches)
	})
}
