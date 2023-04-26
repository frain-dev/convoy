package listener

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/oklog/ulid/v2"
)

type MetaEvent struct {
	Event *datastore.MetaEvent
}

var (
	NoopEndpointListener = NewEndpointListener(mocks.NewMockQueuer(&gomock.Controller{}))
)

type endpointListener struct {
	queue queue.Queuer
}

func NewEndpointListener(queue queue.Queuer) datastore.EndpointListener {
	return &endpointListener{queue: queue}
}

func (e *endpointListener) BeforeCreate(endpoint *datastore.Endpoint) {

}

func (e *endpointListener) AfterCreate(endpoint *datastore.Endpoint) {
	e.metaEvent(datastore.EndpointCreated, endpoint)
}

func (e *endpointListener) BeforeUpdate(endpoint *datastore.Endpoint) {

}

func (e *endpointListener) AfterUpdate(endpoint *datastore.Endpoint) {
	e.metaEvent(datastore.EndpointUpdated, endpoint)
}

func (e *endpointListener) metaEvent(eventType datastore.MetaEventType, endpoint *datastore.Endpoint) {
	endpointByte, err := json.Marshal(endpoint)
	if err != nil {
		log.WithError(err).Error("failed to marshal endpoint")
	}

	data := datastore.MetaEventPayload{
		EventType: string(eventType),
		Data:      endpointByte,
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		log.WithError(err).Error("failed to marshal data")
	}

	event := &datastore.MetaEvent{
		UID:           ulid.Make().String(),
		ProjectID:     endpoint.ProjectID,
		EventType:     eventType,
		Status:        string(datastore.ScheduledEventStatus),
		RetryCount:    1,
		MaxRetryCount: 3,
		Data:          dataByte,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	metaEvent := MetaEvent{
		Event: event,
	}

	metaEventByte, err := json.Marshal(metaEvent)
	if err != nil {
		log.WithError(err).Error("failed to marshal meta event")
	}

	err = e.queue.Write(convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{
		ID:      event.UID,
		Payload: metaEventByte,
	})

	if err != nil {
		log.WithError(err).Error("failed to write to queue")
	}
}
