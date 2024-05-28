package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
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

		cctx := context.WithValue(ctx, postgres.TransactionCtx, tx)

		var isDuplicate bool
		if len(broadcastEvent.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(cctx, broadcastEvent.ProjectID, broadcastEvent.IdempotencyKey)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			isDuplicate = len(events) > 0
		}

		subscriptions, err := subRepo.FetchSubscriptionsForBroadcast(ctx, broadcastEvent.ProjectID, fmt.Sprintf("{%s}", broadcastEvent.EventType), 1000)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("failed to fetch subscriptions with err: %s", err.Error()), delay: defaultDelay}
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
	idMap := make(map[string]string)
	ids := make([]string, 0, len(subs))
	var sub *datastore.Subscription
	for i := range subs {
		sub = &subs[i]
		if sub.Type == datastore.SubscriptionTypeAPI && !util.IsStringEmpty(sub.EndpointID) {
			if _, ok := idMap[sub.EndpointID]; !ok {
				idMap[sub.EndpointID] = sub.EndpointID
				ids = append(ids, sub.EndpointID)
			}
		}
	}

	return ids
}
