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
// FindMetaEventByID Tests
// ============================================================================

func TestFindMetaEventByID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event first
	metaEvent := seedMetaEvent(t, db, project)

	// Find the meta event
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)

	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, metaEvent.UID, fetched.UID)
	require.Equal(t, metaEvent.ProjectID, fetched.ProjectID)
	require.Equal(t, metaEvent.EventType, fetched.EventType)
}

func TestFindMetaEventByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Try to find a non-existent meta event
	fetched, err := service.FindMetaEventByID(ctx, project.UID, ulid.Make().String())

	require.Error(t, err)
	require.Equal(t, datastore.ErrMetaEventNotFound, err)
	require.Nil(t, fetched)
}

func TestFindMetaEventByID_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a meta event
	metaEvent := seedMetaEvent(t, db, project)

	// Try to find with a different project ID
	fetched, err := service.FindMetaEventByID(ctx, ulid.Make().String(), metaEvent.UID)

	require.Error(t, err)
	require.Equal(t, datastore.ErrMetaEventNotFound, err)
	require.Nil(t, fetched)
}

func TestFindMetaEventByID_VerifyAllFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	expectedMetadata := &datastore.Metadata{
		Data:            json.RawMessage(`{"key": "value", "nested": {"inner": true}}`),
		Raw:             `{"key": "value", "nested": {"inner": true}}`,
		Strategy:        datastore.ExponentialStrategyProvider,
		NextSendTime:    time.Now().Add(time.Hour).Truncate(time.Microsecond),
		NumTrials:       5,
		IntervalSeconds: 300,
		RetryLimit:      10,
		MaxRetrySeconds: 7200,
	}

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: string(datastore.EventDeliveryFailed),
		Metadata:  expectedMetadata,
		Status:    datastore.FailureEventStatus,
	}

	err := service.CreateMetaEvent(ctx, metaEvent)
	require.NoError(t, err)

	// Fetch and verify all fields
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)

	require.Equal(t, metaEvent.UID, fetched.UID)
	require.Equal(t, metaEvent.ProjectID, fetched.ProjectID)
	require.Equal(t, metaEvent.EventType, fetched.EventType)
	require.Equal(t, metaEvent.Status, fetched.Status)

	// Verify metadata
	require.NotNil(t, fetched.Metadata)
	require.Equal(t, expectedMetadata.Strategy, fetched.Metadata.Strategy)
	require.Equal(t, expectedMetadata.NumTrials, fetched.Metadata.NumTrials)
	require.Equal(t, expectedMetadata.IntervalSeconds, fetched.Metadata.IntervalSeconds)
	require.Equal(t, expectedMetadata.RetryLimit, fetched.Metadata.RetryLimit)
	require.Equal(t, expectedMetadata.MaxRetrySeconds, fetched.Metadata.MaxRetrySeconds)

	// Verify timestamps are set
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)
}

func TestFindMetaEventByID_WithEmptyMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Note: Database requires non-null metadata, so use empty/minimal metadata
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

	err := service.CreateMetaEvent(ctx, metaEvent)
	require.NoError(t, err)

	// Fetch and verify
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Metadata)
}
