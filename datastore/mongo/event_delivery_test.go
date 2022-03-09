//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_eventDeliveryRepo_UpdateStatusOfEventDeliveries(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err := e.CreateEventDelivery(context.Background(), &delivery1)
	if err != nil {
		require.NoError(t, err)
		return
	}

	delivery2 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery2)
	if err != nil {
		require.NoError(t, err)
		return
	}

	ids := []string{delivery1.UID, delivery2.UID}

	status := datastore.SuccessEventStatus
	err = e.UpdateStatusOfEventDeliveries(context.Background(), ids, status)
	if err != nil {
		require.NoError(t, err)
		return
	}

	d1, err := e.FindEventDeliveryByID(context.Background(), delivery1.UID)
	if err != nil {
		require.NoError(t, err)
		return
	}
	require.Equal(t, status, d1.Status)

	d2, err := e.FindEventDeliveryByID(context.Background(), delivery2.UID)
	if err != nil {
		require.NoError(t, err)
		return
	}

	require.Equal(t, status, d2.Status)
}
