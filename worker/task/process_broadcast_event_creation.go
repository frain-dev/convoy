package task

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/database"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

func ProcessBroadcastEventCreation(db database.Database, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		var broadcastEvent models.BroadcastEvent

		err = msgpack.DecodeMsgPack(t.Payload(), &broadcastEvent)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		project, err := projectRepo.FetchProjectByID(ctx, broadcastEvent.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		tx, err := db.BeginTx(ctx)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}
		defer db.Rollback(tx, err)

		cctx := context.WithValue(ctx, "tx", tx)

		var isDuplicate bool
		if len(broadcastEvent.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(cctx, broadcastEvent.ProjectID, broadcastEvent.IdempotencyKey)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			isDuplicate = len(events) > 0
		}

		pageable := datastore.Pageable{
			PerPage:    3500,
			Direction:  datastore.Next,
			NextCursor: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
		}

		subscriptions, err := getAllSubscriptions(cctx, subRepo, project.UID, pageable)
		if err != nil {
			log.WithError(err).Error("failed to fetch all subscriptions")
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		event := &datastore.Event{
			UID:              ulid.Make().String(),
			EventType:        datastore.EventType(broadcastEvent.EventType),
			ProjectID:        project.UID,
			SourceID:         broadcastEvent.SourceID,
			Data:             broadcastEvent.Data,
			IdempotencyKey:   broadcastEvent.IdempotencyKey,
			Headers:          getCustomHeaders(broadcastEvent.CustomHeaders),
			IsDuplicateEvent: isDuplicate,
			Raw:              string(broadcastEvent.Data),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		subscriptions = matchSubscriptions(string(event.EventType), subscriptions)

		subscriptions, err = matchSubscriptionsUsingFilter(cctx, event, subRepo, subscriptions, true)
		if err != nil {
			return &EndpointError{Err: errors.New("failed to match subscriptions using filter"), delay: defaultDelay}
		}

		event.Endpoints = getEndpointIDs(subscriptions)

		err = eventRepo.CreateEvent(cctx, event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		if event.IsDuplicateEvent {
			log.FromContext(cctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		return writeEventDeliveriesToQueue(
			cctx, subscriptions, event, project, eventDeliveryRepo,
			eventQueue, deviceRepo, endpointRepo,
		)
	}
}

func getEndpointIDs(subs []datastore.Subscription) []string {
	ids := make([]string, 0, len(subs))
	var sub *datastore.Subscription
	for i := range subs {
		sub = &subs[i]
		if sub.Type == datastore.SubscriptionTypeAPI && !util.IsStringEmpty(sub.EndpointID) {
			ids = append(ids, sub.EndpointID)
		}
	}

	return ids
}

func getAllSubscriptions(ctx context.Context, subRepo datastore.SubscriptionRepository, projectID string, pageable datastore.Pageable) ([]datastore.Subscription, error) {
	subscriptions, paginationData, err := subRepo.LoadSubscriptionsPaged(ctx, projectID, &datastore.FilterBy{}, pageable)
	if err != nil {
		return nil, err
	}

	if paginationData.HasNextPage {
		pageable.NextCursor = subscriptions[len(subscriptions)-1].UID
		subs, err := getAllSubscriptions(ctx, subRepo, projectID, pageable)
		if err != nil {
			return nil, err
		}

		subscriptions = append(subscriptions, subs...)

	}

	return subscriptions, nil
}
