package services

import (
	"context"
	"fmt"
	"github.com/oklog/ulid/v2"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateBroadcastEventService struct {
	EndpointRepo   datastore.EndpointRepository
	EventRepo      datastore.EventRepository
	PortalLinkRepo datastore.PortalLinkRepository
	Queue          queue.Queuer

	BroadcastEvent *models.BroadcastEvent
	Project        *datastore.Project
}

func (e *CreateBroadcastEventService) Run(ctx context.Context) error {
	if e.Project == nil {
		return &ServiceError{ErrMsg: "an error occurred while creating broadcast event - invalid project"}
	}

	e.BroadcastEvent.EventID = ulid.Make().String()
	e.BroadcastEvent.ProjectID = e.Project.UID
	jobId := fmt.Sprintf("broadcast:%s:%s", e.BroadcastEvent.ProjectID, e.BroadcastEvent.EventID)
	e.BroadcastEvent.AcknowledgedAt = time.Now()

	taskName := convoy.CreateBroadcastEventProcessor

	eventByte, err := msgpack.EncodeMsgPack(e.BroadcastEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
		Delay:   0,
	}

	err = e.Queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).Errorf("Error occurred sending new broadcast event to the queue %s", err)
		return &ServiceError{ErrMsg: "failed to create dynamic event"}
	}

	return nil
}
