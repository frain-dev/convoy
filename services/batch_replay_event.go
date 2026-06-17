package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

const BatchReplayPageSize = 1000

func NormalizeBatchReplayPageable(pageable datastore.Pageable) datastore.Pageable {
	pageable.PerPage = BatchReplayPageSize
	pageable.Direction = datastore.Next
	pageable.NextCursor = ""
	pageable.PrevCursor = ""
	pageable.SetCursors()
	return pageable
}

type BatchReplayEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer
	EventRepo    datastore.EventRepository

	Filter *datastore.Filter
	Logger log.Logger
}

func (e *BatchReplayEventService) Run(ctx context.Context) (int, int, error) {
	filter := *e.Filter
	filter.Pageable = NormalizeBatchReplayPageable(filter.Pageable)

	rs := ReplayEventService{
		EndpointRepo: e.EndpointRepo,
		Queue:        e.Queue,
		Logger:       e.Logger,
	}

	successes, failures := 0, 0

	for {
		events, pagination, err := e.EventRepo.LoadEventsPaged(ctx, e.Filter.Project.UID, &filter)
		if err != nil {
			e.Logger.ErrorContext(ctx, "failed to fetch events", "error", err, "successes", successes, "failures", failures)
			errMsg := "failed to fetch event deliveries"
			if successes > 0 || failures > 0 {
				errMsg = fmt.Sprintf("batch replay incomplete after %d successful and %d failed replays", successes, failures)
			}
			return successes, failures, &ServiceError{ErrMsg: errMsg, Err: err}
		}

		if len(events) == 0 {
			break
		}

		pageFailures := 0
		for i := range events {
			rs.Event = &events[i]
			if err = rs.Run(ctx); err != nil {
				pageFailures++
				e.Logger.ErrorContext(ctx, "an item in the batch replay failed", "error", err)
			}
		}

		successes += len(events) - pageFailures
		failures += pageFailures

		if !pagination.HasNextPage {
			break
		}

		filter.Pageable.NextCursor = pagination.NextPageCursor
		filter.Pageable.PrevCursor = pagination.PrevPageCursor
	}

	return successes, failures, nil
}
