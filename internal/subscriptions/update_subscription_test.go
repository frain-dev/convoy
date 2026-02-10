package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestUpdateSubscription(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	tests := []struct {
		name    string
		setup   func() *datastore.Subscription
		update  func(*datastore.Subscription) *datastore.Subscription
		wantErr bool
		errMsg  string
		verify  func(*testing.T, *datastore.Subscription)
	}{
		{
			name: "should_update_subscription_name",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.Name = "Updated Subscription Name"
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.Equal(t, "Updated Subscription Name", sub.Name)
			},
		},
		{
			name: "should_change_endpoint_association",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				newEndpoint := seedEndpoint(t, db, project)
				sub.EndpointID = newEndpoint.UID
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotEqual(t, endpoint.UID, sub.EndpointID)
			},
		},
		{
			name: "should_change_source_association",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				newSource := seedSource(t, db, project)
				sub.SourceID = newSource.UID
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotEqual(t, source.UID, sub.SourceID)
			},
		},
		{
			name: "should_update_filter_configuration",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.FilterConfig.Filter.Headers = datastore.M{
					"X-New-Header": "new-value",
					"X-API-Key":    "secret",
				}
				sub.FilterConfig.Filter.Body = datastore.M{
					"status": "active",
					"type":   "premium",
				}
				sub.FilterConfig.Filter.RawHeaders = sub.FilterConfig.Filter.Headers
				sub.FilterConfig.Filter.RawBody = sub.FilterConfig.Filter.Body
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotNil(t, sub.FilterConfig)
				require.NotNil(t, sub.FilterConfig.Filter.Headers)
				require.Equal(t, "new-value", sub.FilterConfig.Filter.Headers["X-New-Header"])
			},
		},
		{
			name: "should_add_event_types",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.FilterConfig.EventTypes = []string{"user.created"}
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.FilterConfig.EventTypes = []string{"user.created", "user.updated", "user.deleted", "order.created"}
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.Len(t, sub.FilterConfig.EventTypes, 4)
				require.Contains(t, sub.FilterConfig.EventTypes, "user.updated")
				require.Contains(t, sub.FilterConfig.EventTypes, "user.deleted")
			},
		},
		{
			name: "should_remove_event_types",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.FilterConfig.EventTypes = []string{"user.created", "user.updated", "user.deleted", "order.created"}
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.FilterConfig.EventTypes = []string{"user.created"}
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.Len(t, sub.FilterConfig.EventTypes, 1)
				require.Equal(t, "user.created", sub.FilterConfig.EventTypes[0])
			},
		},
		{
			name: "should_update_alert_configuration",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.AlertConfig = &datastore.AlertConfiguration{
					Count:     50,
					Threshold: "5h",
				}
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotNil(t, sub.AlertConfig)
				require.Equal(t, 50, sub.AlertConfig.Count)
				require.Equal(t, "5h", sub.AlertConfig.Threshold)
			},
		},
		{
			name: "should_update_retry_configuration",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.RetryConfig = &datastore.RetryConfiguration{
					Type:       datastore.ExponentialStrategyProvider,
					Duration:   120,
					RetryCount: 5,
				}
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotNil(t, sub.RetryConfig)
				require.Equal(t, datastore.ExponentialStrategyProvider, sub.RetryConfig.Type)
				require.Equal(t, uint64(120), sub.RetryConfig.Duration)
				require.Equal(t, uint64(5), sub.RetryConfig.RetryCount)
			},
		},
		{
			name: "should_update_rate_limit_configuration",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.RateLimitConfig = &datastore.RateLimitConfiguration{
					Count:    500,
					Duration: 3600,
				}
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.NotNil(t, sub.RateLimitConfig)
				require.Equal(t, 500, sub.RateLimitConfig.Count)
				require.Equal(t, uint64(3600), sub.RateLimitConfig.Duration)
			},
		},
		{
			name: "should_update_transformation_function",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.Function = null.NewString("function transform(payload) { return {...payload, transformed: true}; }", true)
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.True(t, sub.Function.Valid)
				require.Contains(t, sub.Function.String, "transformed: true")
			},
		},
		{
			name: "should_fail_with_not_found_error",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.UID = ulid.Make().String() // Non-existent subscription
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.Name = "Updated Name"
				return sub
			},
			wantErr: true,
			errMsg:  "not found",
		},
		{
			name: "should_update_delivery_mode",
			setup: func() *datastore.Subscription {
				sub := createTestSubscription(project, endpoint, source)
				sub.DeliveryMode = datastore.AtLeastOnceDeliveryMode
				err := service.CreateSubscription(ctx, project.UID, sub)
				require.NoError(t, err)
				return sub
			},
			update: func(sub *datastore.Subscription) *datastore.Subscription {
				sub.DeliveryMode = datastore.AtMostOnceDeliveryMode
				return sub
			},
			wantErr: false,
			verify: func(t *testing.T, sub *datastore.Subscription) {
				require.Equal(t, datastore.AtMostOnceDeliveryMode, sub.DeliveryMode)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub := tc.setup()
			updated := tc.update(sub)

			err := service.UpdateSubscription(ctx, project.UID, updated)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.Contains(t, err.Error(), tc.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// Verify the update persisted
			fetched, err := service.FindSubscriptionByID(ctx, project.UID, updated.UID)
			require.NoError(t, err)
			require.NotNil(t, fetched)

			// Run custom verification if provided
			if tc.verify != nil {
				tc.verify(t, fetched)
			}

			// Verify timestamps
			require.False(t, fetched.UpdatedAt.IsZero())
			require.True(t, fetched.UpdatedAt.After(fetched.CreatedAt) || fetched.UpdatedAt.Equal(fetched.CreatedAt))
		})
	}
}

func TestUpdateSubscription_ConfigNilHandling(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_set_alert_config_to_nil", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		sub.AlertConfig = nil
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.AlertConfig)
	})

	t.Run("should_set_retry_config_to_nil", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		sub.RetryConfig = nil
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.RetryConfig)
	})

	t.Run("should_set_rate_limit_config_to_nil", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		sub.RateLimitConfig = nil
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.RateLimitConfig)
	})

	t.Run("should_clear_transformation_function", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.Function = null.NewString("function transform(data) { return data; }", true)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		sub.Function = null.NewString("", false)
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.False(t, fetched.Function.Valid)
	})
}

func TestUpdateSubscription_EventTypeFilters(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_update_filters_when_event_types_change", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.FilterConfig.EventTypes = []string{"event.one", "event.two"}
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify initial filters created
		var initialFilterCount int
		err = db.GetConn().QueryRow(ctx, "SELECT COUNT(*) FROM convoy.filters WHERE subscription_id = $1", sub.UID).Scan(&initialFilterCount)
		require.NoError(t, err)
		require.Equal(t, 2, initialFilterCount)

		// Update event types
		sub.FilterConfig.EventTypes = []string{"event.one", "event.three", "event.four"}
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify filters updated
		var updatedFilterCount int
		err = db.GetConn().QueryRow(ctx, "SELECT COUNT(*) FROM convoy.filters WHERE subscription_id = $1", sub.UID).Scan(&updatedFilterCount)
		require.NoError(t, err)
		require.Equal(t, 3, updatedFilterCount)
	})

	t.Run("should_handle_wildcard_event_type_update", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.FilterConfig.EventTypes = []string{"specific.event"}
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Update to wildcard
		sub.FilterConfig.EventTypes = []string{"*"}
		err = service.UpdateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Len(t, fetched.FilterConfig.EventTypes, 1)
		require.Equal(t, "*", fetched.FilterConfig.EventTypes[0])
	})
}
