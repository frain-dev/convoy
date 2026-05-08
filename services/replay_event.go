package services

import (
	"context"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
)

type ReplayEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer

	Event  *datastore.Event
	Logger log.Logger
}

func (e *ReplayEventService) Run(ctx context.Context) error {
	jobId := queue.JobId{ProjectID: e.Event.ProjectID, ResourceID: e.Event.UID}.ReplayJobId()
	createReplayEvent := task.CreateEvent{
		JobID: jobId,
		Event: e.Event,
	}
	createReplayEvent.Event.AcknowledgedAt = null.TimeFrom(time.Now())

	eventByte, err := msgpack.EncodeMsgPack(createReplayEvent)
	if err != nil {
		return &ServiceError{ErrMsg: err.Error()}
	}

	if util.IsStringEmpty(e.Event.UID) || util.IsStringEmpty(e.Event.ProjectID) {
		return &ServiceError{ErrMsg: "missing event or project id"}
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
	}

	err = e.Queue.Write(ctx, convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		e.Logger.ErrorContext(ctx, "replay_event: failed to write event to the queue", "error", err)
		return &ServiceError{ErrMsg: "failed to write event to queue", Err: err}
	}

	return nil
}
