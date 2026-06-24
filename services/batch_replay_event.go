package services

import (
	"context"
	"slices"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

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
	events, _, err := e.EventRepo.LoadEventsPaged(ctx, e.Filter.Project.UID, e.Filter)
	if err != nil {
		e.Logger.ErrorContext(ctx, "failed to fetch events", "error", err)
		return 0, 0, &ServiceError{ErrMsg: "failed to fetch event deliveries", Err: err}
	}

	rs := ReplayEventService{
		EndpointRepo: e.EndpointRepo,
		Queue:        e.Queue,
		Logger:       e.Logger,
	}

	successes, failures := 0, 0
	for _, ev := range events {
		// Count ownership-skipped events as failures so the summary does not over-report
		// successes: a partially foreign multi-endpoint event matches the owned-endpoint
		// filter but must not be replayed (that would redeliver to foreign endpoints).
		if len(e.OwnedEndpointIDs) > 0 && !e.eventFullyOwned(ev) {
			failures++
			e.Logger.WarnContext(ctx, "batch replay skipped event not fully owned by caller", "event_id", ev.UID)
			continue
		}

		rs.Event = &ev
		if err = rs.Run(ctx); err != nil {
			failures++
			e.Logger.ErrorContext(ctx, "an item in the batch replay failed", "error", err)
			continue
		}
		successes++
	}

	return successes, failures, nil
}

func (e *BatchReplayEventService) eventFullyOwned(ev datastore.Event) bool {
	for _, endpointID := range ev.Endpoints {
		if !slices.Contains(e.OwnedEndpointIDs, endpointID) {
			return false
		}
	}
	return true
}
