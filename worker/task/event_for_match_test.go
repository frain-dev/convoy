package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/msgpack"
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

func TestEventForMatchReloadsDynamicMetadataAfterSerialization(t *testing.T) {
	for _, format := range []string{"msgpack", "json"} {
		t.Run(format, func(t *testing.T) {
			event := &datastore.Event{UID: "ev-1", ProjectID: "proj-1", Metadata: `{"dynamicPayload":"synthetic-routing-state"}`}
			original := EventChannelMetadata{Event: event, Config: &EventChannelConfig{Channel: "dynamic"}}
			var decoded EventChannelMetadata
			if format == "msgpack" {
				payload, err := msgpack.EncodeMsgPack(original)
				require.NoError(t, err)
				require.NoError(t, msgpack.DecodeMsgPack(payload, &decoded))
			} else {
				payload, err := json.Marshal(original)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(payload, &decoded))
			}
			require.Empty(t, decoded.Event.Metadata)
			publicJSON, err := json.Marshal(event)
			require.NoError(t, err)
			require.NotContains(t, string(publicJSON), "synthetic-routing-state")
			repo := mocks.NewMockEventRepository(gomock.NewController(t))
			repo.EXPECT().FindEventByID(gomock.Any(), "proj-1", "ev-1").Return(event, nil)
			got, err := eventForMatch(context.Background(), repo, decoded, 0)
			require.NoError(t, err)
			require.Equal(t, event.Metadata, got.Metadata)
		})
	}
}

func TestEventForMatchPropagatesDynamicReloadFailure(t *testing.T) {
	repo := mocks.NewMockEventRepository(gomock.NewController(t))
	want := errors.New("database unavailable")
	repo.EXPECT().FindEventByID(gomock.Any(), "proj-1", "ev-1").Return(nil, want)
	got, err := eventForMatch(context.Background(), repo, EventChannelMetadata{
		Event:  &datastore.Event{UID: "ev-1", ProjectID: "proj-1"},
		Config: &EventChannelConfig{Channel: "dynamic"},
	}, 0)
	require.ErrorIs(t, err, want)
	require.Nil(t, got)
}
