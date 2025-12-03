package services

import (
	"context"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateDynamicEventService struct {
	Queue queue.Queuer

	DynamicEvent *models.DynamicEvent
	Project      *datastore.Project
}

func (e *CreateDynamicEventService) Run(ctx context.Context) error {
	if e.Project == nil {
		return &ServiceError{ErrMsg: "an error occurred while creating dynamic event - invalid project"}
	}
	id := ulid.Make().String()
	jobId := queue.JobId{ProjectID: e.Project.UID, ResourceID: id}.DynamicJobId()

	e.DynamicEvent.EventID = id
	e.DynamicEvent.JobID = jobId
	e.DynamicEvent.ProjectID = e.Project.UID
	e.DynamicEvent.AcknowledgedAt = time.Now()

	if len(e.DynamicEvent.EventTypes) == 0 {
		e.DynamicEvent.EventTypes = []string{"*"}
	}

	taskName := convoy.CreateDynamicEventProcessor

	eventByte, err := msgpack.EncodeMsgPack(e.DynamicEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
	}

	err = e.Queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).Errorf("Error occurred sending new dynamic event to the queue %s", err)
		return &ServiceError{ErrMsg: "failed to create dynamic event"}
	}

	return nil
}
