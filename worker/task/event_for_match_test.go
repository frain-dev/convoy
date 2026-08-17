package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func TestEventForMatchSkipsReloadOnFirstAttempt(t *testing.T) {
	t.Parallel()

	event := &datastore.Event{UID: "ev-1", ProjectID: "proj-1", Status: datastore.PendingStatus}
	got, err := eventForMatch(context.Background(), nil, EventChannelMetadata{Event: event}, 0)
	require.NoError(t, err)
	require.Same(t, event, got)
}

func TestEventForMatchReloadsOnRetry(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEventRepository(ctrl)

	payload := &datastore.Event{UID: "ev-1", ProjectID: "proj-1", Status: datastore.PendingStatus}
	loaded := &datastore.Event{UID: "ev-1", ProjectID: "proj-1", Status: datastore.RetryStatus}

	repo.EXPECT().FindEventByID(gomock.Any(), "proj-1", "ev-1").Return(loaded, nil)

	got, err := eventForMatch(context.Background(), repo, EventChannelMetadata{Event: payload}, 1)
	require.NoError(t, err)
	require.Same(t, loaded, got)
}
