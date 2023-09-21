package listener

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
)

type EndpointListener struct {
	mEvent *services.MetaEvent
}

func NewEndpointListener(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) *EndpointListener {
	mEvent := services.NewMetaEvent(queue, projectRepo, metaEventRepo)
	return &EndpointListener{mEvent: mEvent}
}

func (e *EndpointListener) AfterCreate(data interface{}, _ interface{}) {
	e.metaEvent(string(datastore.EndpointCreated), data)
}

func (e *EndpointListener) AfterUpdate(data interface{}, _ interface{}) {
	e.metaEvent(string(datastore.EndpointUpdated), data)
}

func (e *EndpointListener) AfterDelete(data interface{}, _ interface{}) {
	e.metaEvent(string(datastore.EndpointDeleted), data)
}

func (e *EndpointListener) metaEvent(eventType string, data interface{}) {
	endpoint, ok := data.(*datastore.Endpoint)
	if !ok {
		log.Errorf("invalid type for event - %s", eventType)
		return
	}

	if err := e.mEvent.Run(eventType, endpoint.ProjectID, endpoint); err != nil {
		log.WithError(err).Error("endpoint meta event failed")
	}
}
