package services

import (
	"context"
	"log/slog"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
)

type BatchReplayEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer
	EventRepo    datastore.EventRepository

	Filter *datastore.Filter
}

func (e *BatchReplayEventService) Run(ctx context.Context) (int, int, error) {
	events, _, err := e.EventRepo.LoadEventsPaged(ctx, e.Filter.Project.UID, e.Filter)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch events", "error", err)
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
			slog.ErrorContext(ctx, "an item in the batch replay failed", "error", err)
		}
	}

	successes := len(events) - failures
	return successes, failures, nil
}
