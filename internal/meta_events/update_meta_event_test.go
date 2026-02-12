package meta_events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// UpdateMetaEvent Tests
// ============================================================================

func TestUpdateMetaEvent_ValidUpdate(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event first
	metaEvent := seedMetaEvent(t, db, project)

	// Update the meta event
	metaEvent.EventType = string(datastore.EndpointUpdated)
	metaEvent.Status = datastore.ProcessingEventStatus
	metaEvent.Metadata.NumTrials = 1

	err := service.UpdateMetaEvent(ctx, project.UID, metaEvent)
	require.NoError(t, err)

	// Verify the update
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.Equal(t, string(datastore.EndpointUpdated), fetched.EventType)
	require.Equal(t, datastore.ProcessingEventStatus, fetched.Status)
	require.Equal(t, uint64(1), fetched.Metadata.NumTrials)
}

func TestUpdateMetaEvent_UpdateStatus(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event first
	metaEvent := seedMetaEvent(t, db, project)
	require.Equal(t, datastore.ScheduledEventStatus, metaEvent.Status)

	// Test status transitions
	statuses := []datastore.EventDeliveryStatus{
		datastore.ProcessingEventStatus,
		datastore.SuccessEventStatus,
	}

	for _, status := range statuses {
		metaEvent.Status = status
		err := service.UpdateMetaEvent(ctx, project.UID, metaEvent)
		require.NoError(t, err)

		fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
		require.NoError(t, err)
		require.Equal(t, status, fetched.Status)
	}
}

func TestUpdateMetaEvent_UpdateMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event first
	metaEvent := seedMetaEvent(t, db, project)

	// Update metadata
	newMetadata := &datastore.Metadata{
		Data:            json.RawMessage(`{"updated": true}`),
		Raw:             `{"updated": true}`,
		Strategy:        datastore.LinearStrategyProvider,
		NextSendTime:    time.Now().Add(2 * time.Hour),
		NumTrials:       3,
		IntervalSeconds: 180,
		RetryLimit:      5,
		MaxRetrySeconds: 3600,
	}
	metaEvent.Metadata = newMetadata

	err := service.UpdateMetaEvent(ctx, project.UID, metaEvent)
	require.NoError(t, err)

	// Verify the metadata was updated
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Metadata)
	require.Equal(t, newMetadata.Strategy, fetched.Metadata.Strategy)
	require.Equal(t, newMetadata.NumTrials, fetched.Metadata.NumTrials)
	require.Equal(t, newMetadata.IntervalSeconds, fetched.Metadata.IntervalSeconds)
	require.Equal(t, newMetadata.RetryLimit, fetched.Metadata.RetryLimit)
}

func TestUpdateMetaEvent_WithAttempt(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event first
	metaEvent := seedMetaEvent(t, db, project)
	require.Nil(t, metaEvent.Attempt)

	// Add attempt data
	attempt := &datastore.MetaEventAttempt{
		RequestHeader: datastore.HttpHeader{
			"Content-Type": "application/json",
		},
		ResponseHeader: datastore.HttpHeader{
			"X-Request-Id": "123456",
		},
		ResponseData: `{"success": true}`,
	}
	metaEvent.Attempt = attempt
	metaEvent.Status = datastore.SuccessEventStatus

	err := service.UpdateMetaEvent(ctx, project.UID, metaEvent)
	require.NoError(t, err)

	// Verify the attempt was added
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Attempt)
	require.Equal(t, attempt.ResponseData, fetched.Attempt.ResponseData)
}

func TestUpdateMetaEvent_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Try to update a non-existent meta event
	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: string(datastore.EndpointCreated),
		Metadata: &datastore.Metadata{
			Data:     json.RawMessage(`{}`),
			Strategy: datastore.ExponentialStrategyProvider,
		},
		Status: datastore.ScheduledEventStatus,
	}

	err := service.UpdateMetaEvent(ctx, project.UID, metaEvent)
	require.Error(t, err)
	require.Equal(t, ErrMetaEventNotUpdated, err)
}

func TestUpdateMetaEvent_NilMetaEvent(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	err := service.UpdateMetaEvent(ctx, project.UID, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestUpdateMetaEvent_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event
	metaEvent := seedMetaEvent(t, db, project)

	// Try to update with wrong project ID
	metaEvent.Status = datastore.SuccessEventStatus
	err := service.UpdateMetaEvent(ctx, ulid.Make().String(), metaEvent)
	require.Error(t, err)
	require.Equal(t, ErrMetaEventNotUpdated, err)
}

func TestUpdateMetaEvent_VerifyUpdatedAt(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event
	metaEvent := seedMetaEvent(t, db, project)

	// Get the original updated_at
	original, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	originalUpdatedAt := original.UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update the meta event
	metaEvent.Status = datastore.ProcessingEventStatus
	err = service.UpdateMetaEvent(ctx, project.UID, metaEvent)
	require.NoError(t, err)

	// Verify updated_at changed
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.True(t, fetched.UpdatedAt.After(originalUpdatedAt))
}
