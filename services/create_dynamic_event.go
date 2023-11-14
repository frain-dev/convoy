package services

import (
	"context"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
)

type CreateDynamicEventService struct {
	Queue queue.Queuer

	DynamicEvent *models.DynamicEvent
	Project      *datastore.Project
}

func (e *CreateDynamicEventService) Run(ctx context.Context) error {
	if e.Project == nil {
		return &ServiceError{ErrMsg: "an error occurred while creating event - invalid project"}
	}

	e.DynamicEvent.Event.ProjectID = e.Project.UID

	taskName := convoy.CreateDynamicEventProcessor

	eventByte, err := msgpack.EncodeMsgPack(e.DynamicEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      uuid.NewString(),
		Payload: eventByte,
		Delay:   0,
	}

	err = e.Queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).Errorf("Error occurred sending new dynamic event to the queue %s", err)
		return &ServiceError{ErrMsg: "failed to create dynamic event"}
	}

	return nil
}
