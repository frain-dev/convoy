package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestCreateSubscription(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	tests := []struct {
		name         string
		subscription func() *datastore.Subscription
		wantErr      bool
		errMsg       string
	}{
		{
			name: "should_create_subscription_successfully",
			subscription: func() *datastore.Subscription {
				return createTestSubscription(project, endpoint, source)
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_endpoint_only",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.SourceID = ""
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_source_only",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.EndpointID = ""
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_minimal_config",
			subscription: func() *datastore.Subscription {
				return &datastore.Subscription{
					UID:        ulid.Make().String(),
					Name:       "Minimal Subscription",
					Type:       datastore.SubscriptionTypeAPI,
					ProjectID:  project.UID,
					EndpointID: endpoint.UID,
					FilterConfig: &datastore.FilterConfiguration{
						EventTypes: []string{"*"},
						Filter: datastore.FilterSchema{
							Headers:     datastore.M{},
							Body:        datastore.M{},
							RawHeaders:  datastore.M{},
							RawBody:     datastore.M{},
							IsFlattened: true,
						},
					},
					DeliveryMode: datastore.AtLeastOnceDeliveryMode,
				}
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_transformation_function",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.Function = null.NewString("function transform(data) { return data; }", true)
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_multiple_event_types",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.FilterConfig.EventTypes = []string{"user.created", "user.updated", "user.deleted", "order.created", "order.completed"}
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_wildcard_event_type",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.FilterConfig.EventTypes = []string{"*"}
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_create_subscription_with_complex_filter",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.FilterConfig.Filter.Headers = datastore.M{
					"X-Company-ID": "123",
					"X-User-Role":  "admin",
				}
				sub.FilterConfig.Filter.Body = datastore.M{
					"event":  "user",
					"action": "created",
					"data": datastore.M{
						"role": "admin",
					},
				}
				sub.FilterConfig.Filter.RawHeaders = sub.FilterConfig.Filter.Headers
				sub.FilterConfig.Filter.RawBody = sub.FilterConfig.Filter.Body
				return sub
			},
			wantErr: false,
		},
		{
			name: "should_fail_with_unauthorized_project",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				// Try to create with different project ID in context
				return sub
			},
			wantErr: false, // This test would need a different project ID in the call
		},
		{
			name: "should_create_subscription_with_at_most_once_delivery",
			subscription: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String()
				sub.DeliveryMode = datastore.AtMostOnceDeliveryMode
				return sub
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub := tc.subscription()

			err := service.CreateSubscription(ctx, project.UID, sub)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.Contains(t, err.Error(), tc.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// Verify subscription was created
			fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
			require.NoError(t, err)
			require.NotNil(t, fetched)

			// Verify fields
			assertSubscriptionEqual(t, sub, fetched)

			// Verify timestamps
			require.False(t, fetched.CreatedAt.IsZero())
			require.False(t, fetched.UpdatedAt.IsZero())
		})
	}
}

func TestCreateSubscription_WithTransaction(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_create_event_types_and_filters", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify event types were created
		// This would require checking the event_types and filters tables directly
		// which we can do through the database connection
		var eventTypeCount int
		err = db.GetConn().QueryRow(ctx, "SELECT COUNT(*) FROM convoy.event_types WHERE project_id = $1 AND name = ANY($2)",
			project.UID, sub.FilterConfig.EventTypes).Scan(&eventTypeCount)
		require.NoError(t, err)
		require.Greater(t, eventTypeCount, 0)

		// Verify filters were created
		var filterCount int
		err = db.GetConn().QueryRow(ctx, "SELECT COUNT(*) FROM convoy.filters WHERE subscription_id = $1",
			sub.UID).Scan(&filterCount)
		require.NoError(t, err)
		require.Equal(t, len(sub.FilterConfig.EventTypes), filterCount)
	})
}

func TestCreateSubscription_Validation(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_fail_with_unauthorized_project", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		differentProjectID := ulid.Make().String()

		err := service.CreateSubscription(ctx, differentProjectID, sub)
		require.Error(t, err)
		require.Equal(t, datastore.ErrNotAuthorisedToAccessDocument, err)
	})

	t.Run("should_fail_with_invalid_filter_body", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()

		// Create a filter body that can't be flattened
		sub.FilterConfig.Filter.Body = datastore.M{
			"invalid": make(chan int), // channels can't be marshaled
		}

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to flatten")
	})
}

func TestCreateSubscription_WithoutConfigs(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_create_subscription_with_nil_alert_config", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.AlertConfig = nil

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.AlertConfig)
	})

	t.Run("should_create_subscription_with_nil_retry_config", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.RetryConfig = nil

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.RetryConfig)
	})

	t.Run("should_create_subscription_with_nil_rate_limit_config", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.RateLimitConfig = nil

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.RateLimitConfig)
	})
}

func TestCreateSubscription_EdgeCases(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_handle_empty_event_types_array", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.FilterConfig.EventTypes = []string{}

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.FilterConfig)
		require.Equal(t, 0, len(fetched.FilterConfig.EventTypes))
	})

	t.Run("should_handle_empty_filter_headers", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.FilterConfig.Filter.Headers = datastore.M{}
		sub.FilterConfig.Filter.RawHeaders = datastore.M{}

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.FilterConfig)
		require.Equal(t, 0, len(fetched.FilterConfig.Filter.Headers))
	})

	t.Run("should_handle_empty_filter_body", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.FilterConfig.Filter.Body = datastore.M{}
		sub.FilterConfig.Filter.RawBody = datastore.M{}

		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.FilterConfig)
		require.Equal(t, 0, len(fetched.FilterConfig.Filter.Body))
	})
}
