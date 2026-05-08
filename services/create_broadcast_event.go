package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateBroadcastEventService struct {
	BroadcastEvent *models.BroadcastEvent
	Project        *datastore.Project
	Queue          queue.Queuer
	Logger         log.Logger
}

func (e *CreateBroadcastEventService) Run(ctx context.Context) error {
	if e.Project == nil {
		return &ServiceError{ErrMsg: "an error occurred while creating broadcast event - invalid project"}
	}

	id := ulid.Make().String()
	jobId := queue.JobId{ProjectID: e.Project.UID, ResourceID: id}.BroadcastJobId()

	e.BroadcastEvent.EventID = id
	e.BroadcastEvent.JobID = jobId
	e.BroadcastEvent.ProjectID = e.Project.UID
	e.BroadcastEvent.AcknowledgedAt = time.Now()

	taskName := convoy.CreateBroadcastEventProcessor

	eventByte, err := msgpack.EncodeMsgPack(e.BroadcastEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
	}

	err = e.Queue.Write(ctx, taskName, convoy.CreateEventQueue, job)
	if err != nil {
		e.Logger.ErrorContext(ctx, fmt.Sprintf("Error occurred sending new broadcast event to the queue %s", err))
		return &ServiceError{ErrMsg: "failed to create dynamic event"}
	}

	return nil
}
