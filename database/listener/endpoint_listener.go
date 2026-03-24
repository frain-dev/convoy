package listener

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/frain-dev/convoy/datastore"
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

func (e *EndpointListener) AfterCreate(ctx context.Context, data, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointCreated, data)
}

func (e *EndpointListener) AfterUpdate(ctx context.Context, data, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointUpdated, data)
}

func (e *EndpointListener) AfterDelete(ctx context.Context, data, _ interface{}) {
	e.metaEvent(ctx, datastore.EndpointDeleted, data)
}

func (e *EndpointListener) metaEvent(ctx context.Context, eventType datastore.HookEventType, data interface{}) {
	endpoint, ok := data.(*datastore.Endpoint)
	if !ok {
		slog.Error(fmt.Sprintf("invalid type for event - %s", eventType))
		return
	}

	if err := e.mEvent.Run(ctx, string(eventType), endpoint.ProjectID, endpoint); err != nil {
		slog.Error("endpoint meta event failed", "error", err)
	}
}
