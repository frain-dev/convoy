package listener

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
)

type EndpointListener struct {
	mEvent *services.MetaEvent
	logger log.Logger
}

func NewEndpointListener(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository, logger log.Logger) *EndpointListener {
	mEvent := services.NewMetaEvent(queue, projectRepo, metaEventRepo, logger)
	return &EndpointListener{mEvent: mEvent, logger: logger}
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
		e.logger.Error(fmt.Sprintf("invalid type for event - %s", eventType))
		return
	}

	if err := e.mEvent.Run(ctx, string(eventType), endpoint.ProjectID, endpoint); err != nil {
		e.logger.Error("endpoint meta event failed", "error", err)
	}
}
