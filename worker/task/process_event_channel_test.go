package task

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

func TestMatchSubscriptionsSkipsDuplicateBeforeEmptySubscriptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &EventChannelConfig{Channel: "test-channel", DefaultDelay: time.Second}
	event := &datastore.Event{UID: "event-id-1", ProjectID: "project-id-1", IdempotencyKey: "idempotency-key-1"}
	project := &datastore.Project{UID: "project-id-1"}

	payload, err := msgpack.EncodeMsgPack(EventChannelMetadata{Event: event, Config: cfg})
	require.NoError(t, err)

	fn := MatchSubscriptionsAndCreateEventDeliveries(MatchSubscriptionsDeps{
		Channels: map[string]EventChannel{
			cfg.Channel: &duplicateWithoutSubscriptionsChannel{
				cfg: cfg,
				response: &EventChannelSubResponse{
					Event:            event,
					Project:          project,
					IsDuplicateEvent: true,
				},
			},
		},
		EventRepo: mocks.NewMockEventRepository(ctrl),
		Logger:    log.New("convoy", log.LevelError),
	})

	err = fn(context.Background(), asynq.NewTask("match-subscriptions", payload))
	require.NoError(t, err)
}

type duplicateWithoutSubscriptionsChannel struct {
	cfg      *EventChannelConfig
	response *EventChannelSubResponse
}

func (d *duplicateWithoutSubscriptionsChannel) GetConfig() *EventChannelConfig {
	return d.cfg
}

func (d *duplicateWithoutSubscriptionsChannel) CreateEvent(context.Context, *asynq.Task, EventChannel, EventChannelArgs) (*datastore.Event, error) {
	return nil, nil
}

func (d *duplicateWithoutSubscriptionsChannel) MatchSubscriptions(context.Context, EventChannelMetadata, EventChannelArgs) (*EventChannelSubResponse, error) {
	return d.response, nil
}
