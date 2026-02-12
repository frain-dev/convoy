package subscriptions

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestDeleteSubscription(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_delete_subscription_successfully", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify subscription is deleted (soft delete)
		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.Error(t, err)
		require.Nil(t, fetched)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})

	t.Run("should_verify_soft_delete", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify deleted_at is set in database
		var deletedAt interface{}
		err = db.GetConn().QueryRow(ctx, "SELECT deleted_at FROM convoy.subscriptions WHERE id = $1", sub.UID).Scan(&deletedAt)
		require.NoError(t, err)
		require.NotNil(t, deletedAt, "deleted_at should be set for soft delete")
	})

	t.Run("should_return_error_for_not_found", func(t *testing.T) {
		nonExistentSub := &datastore.Subscription{
			UID:       ulid.Make().String(),
			ProjectID: project.UID,
		}

		err := service.DeleteSubscription(ctx, project.UID, nonExistentSub)
		require.Error(t, err)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})

	t.Run("should_be_idempotent", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// First delete
		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Second delete - should return not found error
		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.Error(t, err)
		require.Equal(t, datastore.ErrSubscriptionNotFound, err)
	})

	t.Run("should_not_return_deleted_subscriptions_in_queries", func(t *testing.T) {
		// Create multiple subscriptions
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		sub3 := createTestSubscription(project, endpoint, source)
		sub3.UID = ulid.Make().String()
		err = service.CreateSubscription(ctx, project.UID, sub3)
		require.NoError(t, err)

		// Delete one subscription
		err = service.DeleteSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		// Verify it's not in FindSubscriptionsByEndpointID
		subs, err := service.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
		require.NoError(t, err)
		for _, sub := range subs {
			require.NotEqual(t, sub2.UID, sub.UID, "Deleted subscription should not be in results")
		}

		// Verify it's not in LoadSubscriptionsPaged
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   100,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		pagedSubs, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		for _, sub := range pagedSubs {
			require.NotEqual(t, sub2.UID, sub.UID, "Deleted subscription should not be in paged results")
		}
	})
}

func TestDeleteSubscription_WithDifferentTypes(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, device := seedTestData(t, db)

	t.Run("should_delete_api_subscription", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.Type = datastore.SubscriptionTypeAPI
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		fetched, err := service.FindSubscriptionByID(ctx, project.UID, sub.UID)
		require.Error(t, err)
		require.Nil(t, fetched)
	})

	t.Run("should_delete_cli_subscription", func(t *testing.T) {
		sub := createTestCLISubscription(project, device)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		err = service.DeleteSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Verify it's not in FindCLISubscriptions
		cliSubs, err := service.FindCLISubscriptions(ctx, project.UID)
		require.NoError(t, err)
		for _, s := range cliSubs {
			require.NotEqual(t, sub.UID, s.UID, "Deleted CLI subscription should not be in results")
		}
	})
}

func TestDeleteSubscription_UnauthorizedProject(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_fail_with_unauthorized_project", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		// Try to delete with different project ID
		differentProjectID := ulid.Make().String()
		err = service.DeleteSubscription(ctx, differentProjectID, sub)
		require.Error(t, err)
	})
}
