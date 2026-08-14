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

func TestMatchSubscriptionsWritesEndpointsAndSucceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &EventChannelConfig{Channel: "test-channel", DefaultDelay: time.Second}
	event := &datastore.Event{UID: "event-id-1", ProjectID: "project-id-1", Data: []byte(`{"ok":true}`)}
	projectCfg := datastore.DefaultProjectConfig
	project := &datastore.Project{UID: "project-id-1", Config: &projectCfg}
	endpoint := &datastore.Endpoint{UID: "endpoint-id-1", Status: datastore.ActiveEndpointStatus}
	sub := datastore.Subscription{
		UID:        "sub-id-1",
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
	}

	payload, err := msgpack.EncodeMsgPack(EventChannelMetadata{Event: event, Config: cfg})
	require.NoError(t, err)

	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventRepo.EXPECT().UpdateEventEndpoints(gomock.Any(), event, []string{endpoint.UID}).Return(nil)
	eventRepo.EXPECT().UpdateEventStatus(gomock.Any(), event, datastore.SuccessStatus, "").Return(nil)

	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	endpointRepo.EXPECT().FindEndpointByID(gomock.Any(), endpoint.UID, project.UID).Return(endpoint, nil)

	deliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	deliveryRepo.EXPECT().CreateEventDeliveries(gomock.Any(), gomock.Any()).Return(nil)

	eventQueue := mocks.NewMockQueuer(ctrl)
	eventQueue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().Transformations().Return(false).AnyTimes()

	fn := MatchSubscriptionsAndCreateEventDeliveries(MatchSubscriptionsDeps{
		Channels: map[string]EventChannel{
			cfg.Channel: &duplicateWithoutSubscriptionsChannel{
				cfg: cfg,
				response: &EventChannelSubResponse{
					Event:         event,
					Project:       project,
					Subscriptions: []datastore.Subscription{sub},
				},
			},
		},
		EventRepo:         eventRepo,
		EndpointRepo:      endpointRepo,
		EventDeliveryRepo: deliveryRepo,
		EventQueue:        eventQueue,
		Licenser:          licenser,
		Logger:            log.New("convoy", log.LevelError),
	})

	err = fn(context.Background(), asynq.NewTask("match-subscriptions", payload))
	require.NoError(t, err)
}

func TestMatchSubscriptionsMissingEndpointIDFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &EventChannelConfig{Channel: "dynamic", DefaultDelay: time.Second}
	event := &datastore.Event{UID: "event-id-1", ProjectID: "project-id-1"}
	project := &datastore.Project{UID: "project-id-1"}
	sub := datastore.Subscription{
		UID:        "sub-id-1",
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: "",
	}

	payload, err := msgpack.EncodeMsgPack(EventChannelMetadata{Event: event, Config: cfg})
	require.NoError(t, err)

	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventRepo.EXPECT().UpdateEventStatus(gomock.Any(), event, datastore.FailureStatus, reasonMissingEndpointID).Return(nil)

	fn := MatchSubscriptionsAndCreateEventDeliveries(MatchSubscriptionsDeps{
		Channels: map[string]EventChannel{
			cfg.Channel: &duplicateWithoutSubscriptionsChannel{
				cfg: cfg,
				response: &EventChannelSubResponse{
					Event:         event,
					Project:       project,
					Subscriptions: []datastore.Subscription{sub},
				},
			},
		},
		EventRepo: eventRepo,
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
