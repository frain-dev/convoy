package listener

import (
	"context"
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

func (e *EndpointListener) AfterCreate(ctx context.Context, data interface{}, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointCreated, data)
}

func (e *EndpointListener) AfterUpdate(ctx context.Context, data interface{}, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointUpdated, data)
}

func (e *EndpointListener) AfterDelete(ctx context.Context, data interface{}, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointDeleted, data)
}

func (e *EndpointListener) metaEvent(ctx context.Context, eventType datastore.HookEventType, data interface{}) {
	endpoint, ok := data.(*datastore.Endpoint)
	if !ok {
		log.Errorf("invalid type for event - %s", eventType)
		return
	}

	if err := e.mEvent.Run(ctx, string(eventType), endpoint.ProjectID, endpoint); err != nil {
		log.WithError(err).Error("endpoint meta event failed")
	}
}
