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
// FindBatchRetryByID Tests
// ============================================================================

func TestFindBatchRetryByID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create a batch retry
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     100,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{"ProjectID": project.UID},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Find by ID
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, batchRetry.ID, fetched.ID)
	require.Equal(t, batchRetry.ProjectID, fetched.ProjectID)
	require.Equal(t, batchRetry.Status, fetched.Status)
}

func TestFindBatchRetryByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createBatchRetryService(t, db)

	// Try to find non-existent batch retry
	fetched, err := service.FindBatchRetryByID(ctx, ulid.Make().String())
	require.Error(t, err)
	require.Nil(t, fetched)
	require.ErrorIs(t, err, datastore.ErrBatchRetryNotFound)
}

// ============================================================================
// FindActiveBatchRetry Tests
// ============================================================================

func TestFindActiveBatchRetry_PendingStatus(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create a pending batch retry
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

	// Find active batch retry
	active, err := service.FindActiveBatchRetry(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, batchRetry.ID, active.ID)
	require.Equal(t, datastore.BatchRetryStatusPending, active.Status)
}

func TestFindActiveBatchRetry_ProcessingStatus(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create a processing batch retry
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

	// Find active batch retry
	active, err := service.FindActiveBatchRetry(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, batchRetry.ID, active.ID)
	require.Equal(t, datastore.BatchRetryStatusProcessing, active.Status)
}

func TestFindActiveBatchRetry_NoActive(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	// Create a completed batch retry (not active)
	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusCompleted,
		TotalEvents:     100,
		ProcessedEvents: 100,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
		CompletedAt:     null.NewTime(now, true),
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Find active batch retry - should return nil, nil
	active, err := service.FindActiveBatchRetry(ctx, project.UID)
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestFindActiveBatchRetry_ReturnsMostRecent(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Create older pending batch retry
	olderRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     50,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now.Add(-10 * time.Minute),
		UpdatedAt:       now.Add(-10 * time.Minute),
	}

	err := service.CreateBatchRetry(ctx, olderRetry)
	require.NoError(t, err)

	// Create newer pending batch retry
	newerRetry := &datastore.BatchRetry{
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

	err = service.CreateBatchRetry(ctx, newerRetry)
	require.NoError(t, err)

	// Find active batch retry - should return the most recent one
	active, err := service.FindActiveBatchRetry(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, newerRetry.ID, active.ID)
}

func TestFindActiveBatchRetry_DifferentProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	project1 := seedProjectForBatchRetry(t, db)
	project2 := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Create batch retry for project1
	batchRetry1 := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project1.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     100,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry1)
	require.NoError(t, err)

	// Find active batch retry for project2 - should return nil
	active, err := service.FindActiveBatchRetry(ctx, project2.UID)
	require.NoError(t, err)
	require.Nil(t, active)

	// Find active batch retry for project1 - should return the created one
	active, err = service.FindActiveBatchRetry(ctx, project1.UID)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, batchRetry1.ID, active.ID)
}
