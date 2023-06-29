package listener

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
	"gopkg.in/guregu/null.v4"
	"time"
)

type EventDeliveryListener struct {
	mEvent *services.MetaEvent
}

type MetaEventDelivery struct {
	UID            string                `json:"uid"`
	ProjectID      string                `json:"project_id,omitempty"`
	EventID        string                `json:"event_id,omitempty"`
	EndpointID     string                `json:"endpoint_id,omitempty"`
	DeviceID       string                `json:"device_id"`
	SubscriptionID string                `json:"subscription_id,omitempty"`
	Headers        httpheader.HTTPHeader `json:"headers"`
	URLQueryParams string                `json:"url_query_params"`
	IdempotencyKey string                `json:"idempotency_key"`

	Endpoint *datastore.Endpoint `json:"endpoint_metadata,omitempty"`
	Event    *datastore.Event    `json:"event_metadata,omitempty"`
	Source   *datastore.Source   `json:"source_metadata,omitempty"`
	Device   *datastore.Device   `json:"device_metadata,omitempty"`

	DeliveryAttempts datastore.DeliveryAttempt     `json:"attempt"`
	Status           datastore.EventDeliveryStatus `json:"status"`
	Metadata         *datastore.Metadata           `json:"metadata"`
	CLIMetadata      *datastore.CLIMetadata        `json:"cli_metadata"`
	Description      string                        `json:"description,omitempty"`
	CreatedAt        time.Time                     `json:"created_at,omitempty"`
	UpdatedAt        time.Time                     `json:"updated_at,omitempty"`
	DeletedAt        null.Time                     `json:"deleted_at,omitempty"`
}

func NewEventDeliveryListener(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) *EventDeliveryListener {
	mEvent := services.NewMetaEvent(queue, projectRepo, metaEventRepo)
	return &EventDeliveryListener{mEvent: mEvent}
}

func (e *EventDeliveryListener) AfterUpdate(data interface{}) {
	eventDelivery, ok := data.(*datastore.EventDelivery)
	if !ok {
		log.Error("invalid type for event - eventdelivery.updated")
		return
	}

	mEventDelivery := getMetaEventDelivery(eventDelivery)
	if len(eventDelivery.DeliveryAttempts) > 0 {
		attempt := eventDelivery.DeliveryAttempts[len(eventDelivery.DeliveryAttempts)-1]
		mEventDelivery.DeliveryAttempts = attempt
	}

	if eventDelivery.Status == datastore.SuccessEventStatus {
		err := e.mEvent.Run(string(datastore.EventDeliverySuccess), eventDelivery.ProjectID, mEventDelivery)
		if err != nil {
			log.WithError(err).Error("event delivery meta event failed")
		}
	}

	if eventDelivery.Status == datastore.FailureEventStatus {
		err := e.mEvent.Run(string(datastore.EventDeliveryFailed), eventDelivery.ProjectID, mEventDelivery)
		if err != nil {
			log.WithError(err).Error("event delivery meta event failed")
		}
	}
}

func getMetaEventDelivery(eventDelivery *datastore.EventDelivery) *MetaEventDelivery {
	return &MetaEventDelivery{
		UID:            eventDelivery.UID,
		ProjectID:      eventDelivery.ProjectID,
		EventID:        eventDelivery.EventID,
		EndpointID:     eventDelivery.EndpointID,
		DeviceID:       eventDelivery.DeviceID,
		SubscriptionID: eventDelivery.SubscriptionID,
		Headers:        eventDelivery.Headers,
		URLQueryParams: eventDelivery.URLQueryParams,
		IdempotencyKey: eventDelivery.IdempotencyKey,
		Endpoint:       eventDelivery.Endpoint,
		Event:          eventDelivery.Event,
		Source:         eventDelivery.Source,
		Device:         eventDelivery.Device,
		Status:         eventDelivery.Status,
		Metadata:       eventDelivery.Metadata,
		CLIMetadata:    eventDelivery.CLIMetadata,
		Description:    eventDelivery.Description,
		CreatedAt:      eventDelivery.CreatedAt,
		UpdatedAt:      eventDelivery.UpdatedAt,
	}
}
