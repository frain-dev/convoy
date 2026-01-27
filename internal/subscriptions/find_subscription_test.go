package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestFindSubscriptionByID(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_find_subscription_by_id", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched)

		assertSubscriptionEqual(t, sub, fetched)
	})

	t.Run("should_return_not_found_error", func(t *testing.T) {
		nonExistentID := ulid.Make().String()

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, nonExistentID)
		require.Error(t, err)
		require.Nil(t, fetched)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})

	t.Run("should_populate_endpoint_metadata", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.Endpoint)
		require.Equal(t, endpoint.UID, fetched.Endpoint.UID)
		require.Equal(t, endpoint.Name, fetched.Endpoint.Name)
		require.Equal(t, endpoint.Url, fetched.Endpoint.Url)
	})

	t.Run("should_populate_source_metadata", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.Source)
		require.Equal(t, source.UID, fetched.Source.UID)
		require.Equal(t, source.Name, fetched.Source.Name)
		require.NotNil(t, fetched.Source.Verifier)
	})

	t.Run("should_handle_subscription_with_endpoint_only", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.SourceID = ""
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.NotNil(t, fetched.Endpoint)
		require.Nil(t, fetched.Source)
	})

	t.Run("should_handle_subscription_with_source_only", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.EndpointID = ""
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.NoError(t, err)
		require.Nil(t, fetched.Endpoint)
		require.NotNil(t, fetched.Source)
	})

	t.Run("should_not_find_deleted_subscription", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.Error(t, err)
		require.Nil(t, fetched)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})
}

func TestFindSubscriptionsBySourceID(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_find_multiple_subscriptions_by_source", func(t *testing.T) {
		// Create multiple subscriptions for the same source
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		sub2.Name = "Second Subscription"
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		sub3 := createTestSubscription(project, endpoint, source)
		sub3.UID = ulid.Make().String()
		sub3.Name = "Third Subscription"
		err = service.CreateSubscription(ctx, project.UID, sub3)
		require.NoError(t, err)

		subscriptions, err := service.FindSubscriptionsBySourceID(ctx, project.UID, source.UID)
		require.NoError(t, err)
		require.Len(t, subscriptions, 3)
	})

	t.Run("should_return_empty_array_for_no_matches", func(t *testing.T) {
		nonExistentSourceID := ulid.Make().String()

		subscriptions, err := service.FindSubscriptionsBySourceID(ctx, project.UID, nonExistentSourceID)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
	})

	t.Run("should_populate_metadata_for_all_subscriptions", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		subscriptions, err := service.FindSubscriptionsBySourceID(ctx, project.UID, source.UID)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		for _, s := range subscriptions {
			require.NotNil(t, s.Source)
			require.Equal(t, source.UID, s.Source.UID)
			if s.EndpointID != "" {
				require.NotNil(t, s.Endpoint)
			}
		}
	})

	t.Run("should_not_return_deleted_subscriptions", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		initialCount := 0
		subscriptions, err := service.FindSubscriptionsBySourceID(ctx, project.UID, source.UID)
		require.NoError(t, err)
		initialCount = len(subscriptions)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		subscriptions, err = service.FindSubscriptionsBySourceID(ctx, project.UID, source.UID)
		require.NoError(t, err)
		require.Equal(t, initialCount-1, len(subscriptions))
	})
}

func TestFindSubscriptionsByEndpointID(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_find_multiple_subscriptions_by_endpoint", func(t *testing.T) {
		// Create multiple subscriptions for the same endpoint
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		sub2.Name = "Second Subscription"
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		subscriptions, err := service.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 2)
	})

	t.Run("should_return_empty_array_for_no_matches", func(t *testing.T) {
		nonExistentEndpointID := ulid.Make().String()

		subscriptions, err := service.FindSubscriptionsByEndpointID(ctx, project.UID, nonExistentEndpointID)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
	})

	t.Run("should_populate_metadata_for_all_subscriptions", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		subscriptions, err := service.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		for _, s := range subscriptions {
			require.NotNil(t, s.Endpoint)
			require.Equal(t, endpoint.UID, s.Endpoint.UID)
			if s.SourceID != "" {
				require.NotNil(t, s.Source)
			}
		}
	})

	t.Run("should_not_return_deleted_subscriptions", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		initialCount := 0
		subscriptions, err := service.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
		require.NoError(t, err)
		initialCount = len(subscriptions)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		subscriptions, err = service.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
		require.NoError(t, err)
		require.Equal(t, initialCount-1, len(subscriptions))
	})
}

func TestFindSubscriptionByDeviceID(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, _, _, device := seedTestData(t, db)

	t.Run("should_find_cli_subscription_by_device_id", func(t *testing.T) {
		sub := createTestCLISubscription(project, device)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByDeviceID(ctx, project.UID, device.UID, datastore.SubscriptionTypeCLI)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		require.Equal(t, sub.UID, fetched.UID)
		require.Equal(t, device.UID, fetched.DeviceID)
		require.Equal(t, datastore.SubscriptionTypeCLI, fetched.Type)
	})

	t.Run("should_return_not_found_error", func(t *testing.T) {
		nonExistentDeviceID := ulid.Make().String()

		fetched, err := service.FindSubscriptionByDeviceID(ctx, project.UID, nonExistentDeviceID, datastore.SubscriptionTypeCLI)
		require.Error(t, err)
		require.Nil(t, fetched)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})

	t.Run("should_populate_device_metadata", func(t *testing.T) {
		sub := createTestCLISubscription(project, device)
		sub.UID = ulid.Make().String()
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByDeviceID(ctx, project.UID, device.UID, datastore.SubscriptionTypeCLI)
		require.NoError(t, err)
		require.NotNil(t, fetched.Device)
		require.Equal(t, device.UID, fetched.Device.UID)
		require.Equal(t, device.HostName, fetched.Device.HostName)
	})

	t.Run("should_filter_by_subscription_type", func(t *testing.T) {
		sub := createTestCLISubscription(project, device)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Try to fetch with wrong type
		fetched, err := service.FindSubscriptionByDeviceID(ctx, project.UID, device.UID, datastore.SubscriptionTypeAPI)
		require.Error(t, err)
		require.Nil(t, fetched)
	})
}

func TestFindCLISubscriptions(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, device := seedTestData(t, db)

	t.Run("should_find_all_cli_subscriptions", func(t *testing.T) {
		// Create CLI subscriptions
		cli1 := createTestCLISubscription(project, device)
		err := service.CreateSubscription(ctx, project.UID, cli1)
		require.NoError(t, err)

		cli2 := createTestCLISubscription(project, device)
		cli2.UID = ulid.Make().String()
		cli2.Name = "Second CLI Subscription"
		err = service.CreateSubscription(ctx, project.UID, cli2)
		require.NoError(t, err)

		// Create API subscription (should be excluded)
		apiSub := createTestSubscription(project, endpoint, source)
		err = service.CreateSubscription(ctx, project.UID, apiSub)
		require.NoError(t, err)

		subscriptions, err := service.FindCLISubscriptions(ctx, project.UID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 2)

		// Verify all returned subscriptions are CLI type
		for _, sub := range subscriptions {
			require.Equal(t, datastore.SubscriptionTypeCLI, sub.Type)
		}
	})

	t.Run("should_return_empty_array_when_no_cli_subscriptions", func(t *testing.T) {
		// Create a new project with no CLI subscriptions
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		newProject := seedProject(t, db, org)

		subscriptions, err := service.FindCLISubscriptions(ctx, newProject.UID)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
	})

	t.Run("should_not_return_deleted_cli_subscriptions", func(t *testing.T) {
		cli := createTestCLISubscription(project, device)
		err := service.CreateSubscription(ctx, project.UID, cli)
		require.NoError(t, err)

		initialCount := 0
		subscriptions, err := service.FindCLISubscriptions(ctx, project.UID)
		require.NoError(t, err)
		initialCount = len(subscriptions)

		err = service.DeleteSubscription(ctx, project.UID, cli)
		require.NoError(t, err)

		subscriptions, err = service.FindCLISubscriptions(ctx, project.UID)
		require.NoError(t, err)
		require.Equal(t, initialCount-1, len(subscriptions))
	})
}
