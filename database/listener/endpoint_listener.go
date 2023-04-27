package listener

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/mevent"
	"github.com/frain-dev/convoy/pkg/log"
)

type endpointListener struct {
	mEvent *mevent.MetaEvent
}

func NewEndpointListener(mEvent *mevent.MetaEvent) datastore.EndpointListener {
	return &endpointListener{mEvent: mEvent}
}

func (e *endpointListener) AfterCreate(endpoint *datastore.Endpoint) {
	err := e.mEvent.Run(datastore.EndpointCreated, endpoint.ProjectID, endpoint)
	if err != nil {
		log.WithError(err).Error("endpoint after create event failed")
	}
}

func (e *endpointListener) AfterUpdate(endpoint *datastore.Endpoint) {
	err := e.mEvent.Run(datastore.EndpointUpdated, endpoint.ProjectID, endpoint)
	if err != nil {
		log.WithError(err).Error("endpoint after update event failed")
	}
}

type noopEndpointListener struct{}

func NewNoopEndpointListener() *noopEndpointListener {
	return &noopEndpointListener{}
}

func (n *noopEndpointListener) AfterCreate(endpoint *datastore.Endpoint) {}
func (n *noopEndpointListener) AfterUpdate(endpoint *datastore.Endpoint) {}
