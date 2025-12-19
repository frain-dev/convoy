package delivery_attempts

import (
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestDeleteProjectDeliveriesAttempts_SoftDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create multiple delivery attempts
	attemptIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		attemptIDs[i] = ulid.Make().String()
		attempt := &datastore.DeliveryAttempt{
			UID:             attemptIDs[i],
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          i%2 == 0,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Verify all attempts were created
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 5)

	// Soft delete all attempts with date range covering all attempts
	now := time.Now()
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: now.Add(-1 * time.Hour).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Hour).Unix(),
	}

	err = service.DeleteProjectDeliveriesAttempts(ctx, project.UID, filter, false)
	require.NoError(t, err)

	// Verify attempts are soft deleted (not returned in queries)
	attempts, err = service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 0, "Soft deleted attempts should not be returned")
}

func TestDeleteProjectDeliveriesAttempts_HardDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create multiple delivery attempts
	attemptIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		attemptIDs[i] = ulid.Make().String()
		attempt := &datastore.DeliveryAttempt{
			UID:             attemptIDs[i],
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Hard delete all attempts
	now := time.Now()
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: now.Add(-1 * time.Hour).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Hour).Unix(),
	}

	err := service.DeleteProjectDeliveriesAttempts(ctx, project.UID, filter, true)
	require.NoError(t, err)

	// Verify attempts are permanently deleted
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 0, "Hard deleted attempts should be permanently removed")
}

func TestDeleteProjectDeliveriesAttempts_DateRangeFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create attempts (all will have similar timestamps in test)
	for i := 0; i < 5; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Delete with a date range that covers all attempts
	now := time.Now()
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: now.Add(-2 * time.Hour).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Hour).Unix(),
	}

	err := service.DeleteProjectDeliveriesAttempts(ctx, project.UID, filter, false)
	require.NoError(t, err)

	// Verify all attempts were deleted
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 0)
}

func TestDeleteProjectDeliveriesAttempts_NoMatchingAttempts(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create an attempt
	attempt := &datastore.DeliveryAttempt{
		UID:             ulid.Make().String(),
		URL:             "https://example.com/webhook",
		Method:          "POST",
		APIVersion:      "2023.12.25",
		EndpointID:      endpoint.UID,
		EventDeliveryId: eventDelivery.UID,
		ProjectId:       project.UID,
		Status:          true,
	}
	err := service.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	// Try to delete with a date range that doesn't match
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: time.Now().Add(-2 * time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(-1 * time.Hour).Unix(),
	}

	err = service.DeleteProjectDeliveriesAttempts(ctx, project.UID, filter, false)
	require.Error(t, err)
	require.Equal(t, datastore.ErrDeliveryAttemptsNotDeleted, err)

	// Verify attempt still exists
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
}

func TestDeleteProjectDeliveriesAttempts_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create an attempt
	attempt := &datastore.DeliveryAttempt{
		UID:             ulid.Make().String(),
		URL:             "https://example.com/webhook",
		Method:          "POST",
		APIVersion:      "2023.12.25",
		EndpointID:      endpoint.UID,
		EventDeliveryId: eventDelivery.UID,
		ProjectId:       project.UID,
		Status:          true,
	}
	err := service.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	// Try to delete with wrong project ID
	now := time.Now()
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: now.Add(-1 * time.Hour).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Hour).Unix(),
	}

	wrongProjectID := ulid.Make().String()
	err = service.DeleteProjectDeliveriesAttempts(ctx, wrongProjectID, filter, false)
	require.Error(t, err)
	require.Equal(t, datastore.ErrDeliveryAttemptsNotDeleted, err)

	// Verify attempt still exists
	attempts, err := service.FindDeliveryAttempts(ctx, eventDelivery.UID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
}

func TestDeleteProjectDeliveriesAttempts_NilFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)

	service := New(log.NewLogger(os.Stdout), db)

	// Try to delete with nil filter
	err := service.DeleteProjectDeliveriesAttempts(ctx, project.UID, nil, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "filter cannot be nil")
}

func TestDeleteProjectDeliveriesAttempts_MultipleProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	// Create two projects
	project1 := seedTestData(t, db, ctx)
	project2 := seedTestData(t, db, ctx)

	endpoint1 := seedEndpoint(t, db, ctx, project1)
	endpoint2 := seedEndpoint(t, db, ctx, project2)

	eventDelivery1 := seedEventDelivery(t, db, ctx, project1, endpoint1)
	eventDelivery2 := seedEventDelivery(t, db, ctx, project2, endpoint2)

	service := New(log.NewLogger(os.Stdout), db)

	// Create attempts for both projects
	for i := 0; i < 3; i++ {
		attempt1 := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint1.UID,
			EventDeliveryId: eventDelivery1.UID,
			ProjectId:       project1.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt1)
		require.NoError(t, err)

		attempt2 := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint2.UID,
			EventDeliveryId: eventDelivery2.UID,
			ProjectId:       project2.UID,
			Status:          true,
		}
		err = service.CreateDeliveryAttempt(ctx, attempt2)
		require.NoError(t, err)
	}

	// Delete attempts for project1 only
	now := time.Now()
	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: now.Add(-1 * time.Hour).Unix(),
		CreatedAtEnd:   now.Add(1 * time.Hour).Unix(),
	}

	err := service.DeleteProjectDeliveriesAttempts(ctx, project1.UID, filter, false)
	require.NoError(t, err)

	// Verify project1 attempts are deleted
	attempts1, err := service.FindDeliveryAttempts(ctx, eventDelivery1.UID)
	require.NoError(t, err)
	require.Len(t, attempts1, 0, "Project1 attempts should be deleted")

	// Verify project2 attempts are still there
	attempts2, err := service.FindDeliveryAttempts(ctx, eventDelivery2.UID)
	require.NoError(t, err)
	require.Len(t, attempts2, 3, "Project2 attempts should remain")
}
