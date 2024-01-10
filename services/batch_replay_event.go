package services

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type BatchReplayEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer
	EventRepo    datastore.EventRepository
	Project      *datastore.Project

	Filter *datastore.EventFilter
}

func (e *BatchReplayEventService) Run(ctx context.Context) (int, int, error) {
	events, _, err := e.EventRepo.LoadEventsPaged(ctx, e.Project.UID, e.Filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events")
		return 0, 0, &ServiceError{ErrMsg: "failed to fetch event deliveries", Err: err}
	}

	rs := ReplayEventService{
		EndpointRepo: e.EndpointRepo,
		Queue:        e.Queue,
	}

	failures := 0
	for _, ev := range events {
		rs.Event = &ev
		err = rs.Run(ctx)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the batch replay failed")
		}
	}

	successes := len(events) - failures
	return successes, failures, nil
}
