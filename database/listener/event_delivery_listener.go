package listener

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
)

type EventDeliveryListener struct {
	mEvent *services.MetaEvent
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

	if eventDelivery.Status == datastore.SuccessEventStatus {
		err := e.mEvent.Run(string(datastore.EventDeliverySuccess), eventDelivery.ProjectID, eventDelivery)
		if err != nil {
			log.WithError(err).Error("event delivery meta event failed")
		}
	}

	if eventDelivery.Status == datastore.FailureEventStatus {
		err := e.mEvent.Run(string(datastore.EventDeliveryFailed), eventDelivery.ProjectID, eventDelivery)
		if err != nil {
			log.WithError(err).Error("event delivery meta event failed")
		}
	}
}
