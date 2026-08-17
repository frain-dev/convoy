package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func TestEventDeliveryForDispatchSkipsReloadOnFirstAttempt(t *testing.T) {
	t.Parallel()

	snapshot := &datastore.EventDelivery{
		UID:          "del-1",
		ProjectID:    "proj-1",
		Status:       datastore.ScheduledEventStatus,
		DeliveryMode: datastore.AtLeastOnceDeliveryMode,
		Metadata:     &datastore.Metadata{RetryLimit: 3},
	}
	data := EventDelivery{
		EventDeliveryID: "del-1",
		ProjectID:       "proj-1",
		Snapshot:        snapshot,
	}

	got, err := eventDeliveryForDispatch(context.Background(), nil, data, 0)
	require.NoError(t, err)
	require.Same(t, snapshot, got)
}

func TestEventDeliveryForDispatchReloadsOnRetry(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEventDeliveryRepository(ctrl)

	snapshot := &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1", Status: datastore.ScheduledEventStatus, DeliveryMode: datastore.AtLeastOnceDeliveryMode}
	loaded := &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1", Status: datastore.RetryEventStatus}
	data := EventDelivery{
		EventDeliveryID: "del-1",
		ProjectID:       "proj-1",
		Snapshot:        snapshot,
	}

	repo.EXPECT().FindEventDeliveryByIDSlim(gomock.Any(), "proj-1", "del-1").Return(loaded, nil)

	got, err := eventDeliveryForDispatch(context.Background(), repo, data, 1)
	require.NoError(t, err)
	require.Same(t, loaded, got)
}

// A reclaimed job comes back with retry count 0, so at-most-once must see the
// live Processing row rather than the pre-dispatch snapshot.
func TestEventDeliveryForDispatchReloadsForAtMostOnce(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEventDeliveryRepository(ctrl)

	data := EventDelivery{
		EventDeliveryID: "del-1",
		ProjectID:       "proj-1",
		Snapshot: &datastore.EventDelivery{
			UID:          "del-1",
			ProjectID:    "proj-1",
			Status:       datastore.ScheduledEventStatus,
			DeliveryMode: datastore.AtMostOnceDeliveryMode,
		},
	}
	loaded := &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1", Status: datastore.ProcessingEventStatus}

	repo.EXPECT().FindEventDeliveryByIDSlim(gomock.Any(), "proj-1", "del-1").Return(loaded, nil)

	got, err := eventDeliveryForDispatch(context.Background(), repo, data, 0)
	require.NoError(t, err)
	require.Same(t, loaded, got)
}

func TestEventDeliveryForDispatchReloadsWhenDeliveryModeUnset(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEventDeliveryRepository(ctrl)

	data := EventDelivery{
		EventDeliveryID: "del-1",
		ProjectID:       "proj-1",
		Snapshot:        &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1"},
	}
	loaded := &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1", Status: datastore.ProcessingEventStatus}

	repo.EXPECT().FindEventDeliveryByIDSlim(gomock.Any(), "proj-1", "del-1").Return(loaded, nil)

	got, err := eventDeliveryForDispatch(context.Background(), repo, data, 0)
	require.NoError(t, err)
	require.Same(t, loaded, got)
}

func TestEventDeliveryForDispatchReloadsWhenSnapshotIDsMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEventDeliveryRepository(ctrl)

	data := EventDelivery{
		EventDeliveryID: "del-1",
		ProjectID:       "proj-1",
		Snapshot:        &datastore.EventDelivery{UID: "other", ProjectID: "proj-1", DeliveryMode: datastore.AtLeastOnceDeliveryMode},
	}
	loaded := &datastore.EventDelivery{UID: "del-1", ProjectID: "proj-1"}

	repo.EXPECT().FindEventDeliveryByIDSlim(gomock.Any(), "proj-1", "del-1").Return(loaded, nil)

	got, err := eventDeliveryForDispatch(context.Background(), repo, data, 0)
	require.NoError(t, err)
	require.Same(t, loaded, got)
}
