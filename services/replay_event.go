package services

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

type ReplayEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer

	Event *datastore.Event
}

func (e *ReplayEventService) Run(ctx context.Context) error {
	taskName := convoy.CreateEventProcessor

	createEvent := task.CreateEvent{
		Event: *e.Event,
	}

	eventByte, err := json.Marshal(createEvent)
	if err != nil {
		return &ServiceError{ErrMsg: err.Error()}
	}

	payload := json.RawMessage(eventByte)

	job := &queue.Job{
		ID:      e.Event.UID,
		Payload: payload,
		Delay:   0,
	}

	err = e.Queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("replay_event: failed to write event to the queue")
		return &ServiceError{ErrMsg: "failed to write event to queue", Err: err}
	}

	return nil
}
