package listener

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/mevent"
	"github.com/frain-dev/convoy/pkg/log"
)

type EndpointListener struct {
	mEvent *mevent.MetaEvent
}

func NewEndpointListener(mEvent *mevent.MetaEvent) *EndpointListener {
	return &EndpointListener{mEvent: mEvent}
}

func (e *EndpointListener) AfterCreate(data interface{}) {
	e.metaEvent(string(datastore.EndpointCreated), data)
}

func (e *EndpointListener) AfterUpdate(data interface{}) {
	e.metaEvent(string(datastore.EndpointUpdated), data)
}

func (e *EndpointListener) AfterDelete(data interface{}) {
	e.metaEvent(string(datastore.EndpointDeleted), data)
}

func (e *EndpointListener) metaEvent(eventType string, data interface{}) {
	endpoint, ok := data.(*datastore.Endpoint)
	if !ok {
		log.Error("invalid type")
	}

	err := e.mEvent.Run(eventType, endpoint.ProjectID, endpoint)
	if err != nil {
		log.WithError(err).Error("endpoint meta event failed")
	}
}
