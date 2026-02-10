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
// CreateMetaEvent Tests
// ============================================================================

func TestCreateMetaEvent_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: string(datastore.EndpointCreated),
		Metadata: &datastore.Metadata{
			Data:            json.RawMessage(`{"endpoint_id": "123"}`),
			Raw:             `{"endpoint_id": "123"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       0,
			IntervalSeconds: 60,
			RetryLimit:      3,
		},
		Status: datastore.ScheduledEventStatus,
	}

	err := service.CreateMetaEvent(ctx, metaEvent)

	require.NoError(t, err)

	// Verify the meta event was created
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.Equal(t, metaEvent.UID, fetched.UID)
	require.Equal(t, metaEvent.ProjectID, fetched.ProjectID)
	require.Equal(t, metaEvent.EventType, fetched.EventType)
	require.Equal(t, metaEvent.Status, fetched.Status)
}

func TestCreateMetaEvent_WithMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	expectedMetadata := &datastore.Metadata{
		Data:            json.RawMessage(`{"endpoint_id": "456", "event": "created"}`),
		Raw:             `{"endpoint_id": "456", "event": "created"}`,
		Strategy:        datastore.LinearStrategyProvider,
		NextSendTime:    time.Now().Add(2 * time.Hour),
		NumTrials:       2,
		IntervalSeconds: 120,
		RetryLimit:      5,
		MaxRetrySeconds: 3600,
	}

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: string(datastore.EventDeliverySuccess),
		Metadata:  expectedMetadata,
		Status:    datastore.SuccessEventStatus,
	}

	err := service.CreateMetaEvent(ctx, metaEvent)
	require.NoError(t, err)

	// Verify metadata was stored correctly
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Metadata)
	require.Equal(t, expectedMetadata.Strategy, fetched.Metadata.Strategy)
	require.Equal(t, expectedMetadata.NumTrials, fetched.Metadata.NumTrials)
	require.Equal(t, expectedMetadata.IntervalSeconds, fetched.Metadata.IntervalSeconds)
	require.Equal(t, expectedMetadata.RetryLimit, fetched.Metadata.RetryLimit)
}

func TestCreateMetaEvent_DifferentEventTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	eventTypes := []datastore.HookEventType{
		datastore.EndpointCreated,
		datastore.EndpointUpdated,
		datastore.EndpointDeleted,
		datastore.EventDeliverySuccess,
		datastore.EventDeliveryFailed,
		datastore.EventDeliveryUpdated,
		datastore.ProjectUpdated,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			metaEvent := &datastore.MetaEvent{
				UID:       ulid.Make().String(),
				ProjectID: project.UID,
				EventType: string(eventType),
				Metadata: &datastore.Metadata{
					Data:     json.RawMessage(`{}`),
					Strategy: datastore.ExponentialStrategyProvider,
				},
				Status: datastore.ScheduledEventStatus,
			}

			err := service.CreateMetaEvent(ctx, metaEvent)
			require.NoError(t, err)

			fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
			require.NoError(t, err)
			require.Equal(t, string(eventType), fetched.EventType)
		})
	}
}

func TestCreateMetaEvent_DifferentStatuses(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	statuses := []datastore.EventDeliveryStatus{
		datastore.ScheduledEventStatus,
		datastore.ProcessingEventStatus,
		datastore.SuccessEventStatus,
		datastore.FailureEventStatus,
		datastore.RetryEventStatus,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			metaEvent := &datastore.MetaEvent{
				UID:       ulid.Make().String(),
				ProjectID: project.UID,
				EventType: string(datastore.EndpointCreated),
				Metadata: &datastore.Metadata{
					Data:     json.RawMessage(`{}`),
					Strategy: datastore.ExponentialStrategyProvider,
				},
				Status: status,
			}

			err := service.CreateMetaEvent(ctx, metaEvent)
			require.NoError(t, err)

			fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
			require.NoError(t, err)
			require.Equal(t, status, fetched.Status)
		})
	}
}

func TestCreateMetaEvent_NilMetaEvent(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createMetaEventService(t, db)

	err := service.CreateMetaEvent(ctx, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestCreateMetaEvent_VerifyTimestamps(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	beforeCreate := time.Now()

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

	afterCreate := time.Now()

	// Verify timestamps
	fetched, err := service.FindMetaEventByID(ctx, project.UID, metaEvent.UID)
	require.NoError(t, err)
	require.True(t, fetched.CreatedAt.After(beforeCreate.Add(-time.Second)))
	require.True(t, fetched.CreatedAt.Before(afterCreate.Add(time.Second)))
	require.NotZero(t, fetched.UpdatedAt)
}
