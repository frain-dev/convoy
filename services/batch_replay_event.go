package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

const BatchReplayPageSize = 1000

// NormalizeBatchReplayPageable ignores caller page size, direction, and cursors.
// Batch replay paginates internally so the replay window matches countbatchreplayevents.
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
	// OwnedEndpointIDs, when non-empty, restricts replay to events whose endpoints are
	// all in the set. Portal-link callers set it so replaying a multi-endpoint event
	// cannot redeliver to endpoints the caller does not own. Empty means no restriction.
	OwnedEndpointIDs []string
	Logger           log.Logger
}

func (e *BatchReplayEventService) Run(ctx context.Context) (int, int, error) {
	filter := *e.Filter
	filter.Pageable = NormalizeBatchReplayPageable(filter.Pageable)

	rs := ReplayEventService{
		EndpointRepo: e.EndpointRepo,
		Queue:        e.Queue,
		Logger:       e.Logger,
	}

	events, pagination, err := e.EventRepo.LoadEventsPaged(ctx, e.Filter.Project.UID, &filter)
	if err != nil {
		return e.fetchError(ctx, err, 0, 0)
	}

	successes, failures := 0, 0

	for len(events) > 0 {
		// Prefetch the next page before enqueueing this one so a fetch error
		// cannot leave this page queued while the handler still returns a
		// retryable status. If the prefetch fails, replay the page already in
		// hand, then return incomplete (409 once any job landed).
		if pagination.HasNextPage {
			filter.Pageable.NextCursor = pagination.NextPageCursor
			filter.Pageable.PrevCursor = pagination.PrevPageCursor
			nextEvents, nextPagination, nextErr := e.EventRepo.LoadEventsPaged(ctx, e.Filter.Project.UID, &filter)
			if nextErr != nil {
				s, f := e.replayPage(ctx, &rs, events)
				successes += s
				failures += f
				return e.fetchError(ctx, nextErr, successes, failures)
			}

			s, f := e.replayPage(ctx, &rs, events)
			successes += s
			failures += f
			events = nextEvents
			pagination = nextPagination
			continue
		}

		s, f := e.replayPage(ctx, &rs, events)
		successes += s
		failures += f
		break
	}

	return successes, failures, nil
}

func (e *BatchReplayEventService) replayPage(ctx context.Context, rs *ReplayEventService, events []datastore.Event) (int, int) {
	pageFailures := 0
	for i := range events {
		// Count ownership-skipped events as failures so the summary does not over-report
		// successes: a partially foreign multi-endpoint event matches the owned-endpoint
		// filter but must not be replayed (that would redeliver to foreign endpoints).
		if len(e.OwnedEndpointIDs) > 0 && !e.eventFullyOwned(events[i]) {
			pageFailures++
			e.Logger.WarnContext(ctx, "batch replay skipped event not fully owned by caller", "event_id", events[i].UID)
			continue
		}

		rs.Event = &events[i]
		if err := rs.Run(ctx); err != nil {
			pageFailures++
			e.Logger.ErrorContext(ctx, "an item in the batch replay failed", "error", err)
		}
	}

	return len(events) - pageFailures, pageFailures
}

func (e *BatchReplayEventService) fetchError(ctx context.Context, err error, successes, failures int) (int, int, error) {
	e.Logger.ErrorContext(ctx, "failed to fetch events", "error", err, "successes", successes, "failures", failures)
	errMsg := "failed to fetch event deliveries"
	if successes > 0 || failures > 0 {
		errMsg = fmt.Sprintf("batch replay incomplete after %d successful and %d failed replays", successes, failures)
	}
	return successes, failures, &ServiceError{ErrMsg: errMsg, Err: err}
}

func (e *BatchReplayEventService) eventFullyOwned(ev datastore.Event) bool {
	for _, endpointID := range ev.Endpoints {
		if !slices.Contains(e.OwnedEndpointIDs, endpointID) {
			return false
		}
	}
	return true
}
