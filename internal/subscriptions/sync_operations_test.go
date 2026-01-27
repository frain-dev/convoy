package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestFetchSubscriptionsForBroadcast(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_match_single_event_type", func(t *testing.T) {
		// Create subscriptions with specific event types
		sub1 := createTestSubscription(project, endpoint, source)
		sub1.FilterConfig.EventTypes = []string{"user.created"}
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		sub2.FilterConfig.EventTypes = []string{"order.created"}
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		// Fetch subscriptions for user.created event
		eventType := "user.created"
		subscriptions, err := service.FetchSubscriptionsForBroadcast(ctx, project.UID, eventType, 10)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		// Verify sub1 is in results
		found := false
		for _, sub := range subscriptions {
			if sub.UID == sub1.UID {
				found = true
				break
			}
		}
		require.True(t, found)
	})

	t.Run("should_match_wildcard_event_type", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.FilterConfig.EventTypes = []string{"*"}
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Fetch subscriptions for any event
		eventType := "some.random.event"
		subscriptions, err := service.FetchSubscriptionsForBroadcast(ctx, project.UID, eventType, 10)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		// Verify wildcard subscription is in results
		found := false
		for _, s := range subscriptions {
			if s.UID == sub.UID {
				found = true
				break
			}
		}
		require.True(t, found)
	})

	t.Run("should_match_multiple_subscriptions", func(t *testing.T) {
		// Create multiple subscriptions for the same event type
		for i := 0; i < 5; i++ {
			sub := createTestSubscription(project, endpoint, source)
			sub.UID = ulid.Make().String()
			sub.FilterConfig.EventTypes = []string{"notification.sent"}
			err := service.CreateSubscription(ctx, project.UID, sub)
			require.NoError(t, err)
		}

		eventType := "notification.sent"
		subscriptions, err := service.FetchSubscriptionsForBroadcast(ctx, project.UID, eventType, 10)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 5)
	})

	t.Run("should_return_empty_for_no_matches", func(t *testing.T) {
		eventType := "non.existent.event"
		subscriptions, err := service.FetchSubscriptionsForBroadcast(ctx, project.UID, eventType, 10)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
	})
}

func TestLoadAllSubscriptionConfig(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_load_across_multiple_projects", func(t *testing.T) {
		// Create subscriptions in first project
		for i := 0; i < 3; i++ {
			sub := createTestSubscription(project, endpoint, source)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, project.UID, sub)
			require.NoError(t, err)
		}

		// Create second project with subscriptions
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		project2 := seedProject(t, db, org)
		endpoint2 := seedEndpoint(t, db, project2)
		source2 := seedSource(t, db, project2)

		for i := 0; i < 2; i++ {
			sub := createTestSubscription(project2, endpoint2, source2)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, project2.UID, sub)
			require.NoError(t, err)
		}

		// Load all subscriptions from both projects
		projectIDs := []string{project.UID, project2.UID}
		subscriptions, err := service.LoadAllSubscriptionConfig(ctx, projectIDs, 100)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 5)
	})

	t.Run("should_handle_empty_projects_list", func(t *testing.T) {
		projectIDs := []string{}
		subscriptions, err := service.LoadAllSubscriptionConfig(ctx, projectIDs, 100)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
	})

	t.Run("should_handle_large_dataset", func(t *testing.T) {
		// Create 20 subscriptions
		for i := 0; i < 20; i++ {
			sub := createTestSubscription(project, endpoint, source)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, project.UID, sub)
			require.NoError(t, err)
		}

		projectIDs := []string{project.UID}
		subscriptions, err := service.LoadAllSubscriptionConfig(ctx, projectIDs, 100)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 20)
	})
}

// Note: FetchNewSubscriptions, FetchDeletedSubscriptions, and FetchUpdatedSubscriptions
// have more complex signatures involving subscriptionUpdates and project arrays.
// These would require additional test infrastructure setup.
// Tests for these methods can be added once their implementation is finalized.
