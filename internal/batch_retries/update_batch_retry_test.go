package batch_retries

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// UpdateBatchRetry Tests
// ============================================================================

func TestUpdateBatchRetry_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create initial batch retry
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     100,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Update the batch retry
	updatedAt := now.Add(1 * time.Minute)
	batchRetry.Status = datastore.BatchRetryStatusProcessing
	batchRetry.ProcessedEvents = 50
	batchRetry.UpdatedAt = updatedAt

	err = service.UpdateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify the update
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.Equal(t, datastore.BatchRetryStatusProcessing, fetched.Status)
	require.Equal(t, 50, fetched.ProcessedEvents)
}

func TestUpdateBatchRetry_ToCompleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create initial batch retry
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusProcessing,
		TotalEvents:     100,
		ProcessedEvents: 50,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Update to completed
	completedAt := now.Add(5 * time.Minute)
	batchRetry.Status = datastore.BatchRetryStatusCompleted
	batchRetry.ProcessedEvents = 100
	batchRetry.UpdatedAt = completedAt
	batchRetry.CompletedAt = null.NewTime(completedAt, true)

	err = service.UpdateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify the update
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.Equal(t, datastore.BatchRetryStatusCompleted, fetched.Status)
	require.Equal(t, 100, fetched.ProcessedEvents)
	require.True(t, fetched.CompletedAt.Valid)
}

func TestUpdateBatchRetry_ToFailed(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create initial batch retry
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusProcessing,
		TotalEvents:     100,
		ProcessedEvents: 30,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Update to failed
	failedAt := now.Add(3 * time.Minute)
	batchRetry.Status = datastore.BatchRetryStatusFailed
	batchRetry.ProcessedEvents = 50
	batchRetry.FailedEvents = 20
	batchRetry.Error = "processing failed: timeout"
	batchRetry.UpdatedAt = failedAt
	batchRetry.CompletedAt = null.NewTime(failedAt, true)

	err = service.UpdateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify the update
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.Equal(t, datastore.BatchRetryStatusFailed, fetched.Status)
	require.Equal(t, 50, fetched.ProcessedEvents)
	require.Equal(t, 20, fetched.FailedEvents)
	require.Equal(t, "processing failed: timeout", fetched.Error)
}

func TestUpdateBatchRetry_NilBatchRetry(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createBatchRetryService(t, db)

	err := service.UpdateBatchRetry(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestUpdateBatchRetry_NonExistentID(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(), // Non-existent ID
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusCompleted,
		TotalEvents:     100,
		ProcessedEvents: 100,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.UpdateBatchRetry(ctx, batchRetry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no rows affected")
}

func TestUpdateBatchRetry_IncrementalProgress(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create initial batch retry
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     100,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Simulate incremental progress updates
	updates := []struct {
		processed int
		failed    int
		status    datastore.BatchRetryStatus
	}{
		{25, 0, datastore.BatchRetryStatusProcessing},
		{50, 5, datastore.BatchRetryStatusProcessing},
		{75, 10, datastore.BatchRetryStatusProcessing},
		{90, 10, datastore.BatchRetryStatusCompleted},
	}

	for i, update := range updates {
		batchRetry.ProcessedEvents = update.processed
		batchRetry.FailedEvents = update.failed
		batchRetry.Status = update.status
		batchRetry.UpdatedAt = now.Add(time.Duration(i+1) * time.Minute)

		if update.status == datastore.BatchRetryStatusCompleted {
			batchRetry.CompletedAt = null.NewTime(batchRetry.UpdatedAt, true)
		}

		err = service.UpdateBatchRetry(ctx, batchRetry)
		require.NoError(t, err)

		// Verify each update
		fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
		require.NoError(t, err)
		require.Equal(t, update.processed, fetched.ProcessedEvents)
		require.Equal(t, update.failed, fetched.FailedEvents)
		require.Equal(t, update.status, fetched.Status)
	}
}
