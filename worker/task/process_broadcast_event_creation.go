package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

var ErrFailedToWriteToQueue = errors.New("failed to write to event delivery queue")

func ProcessBroadcastEventCreation(db database.Database, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		var broadcastEvent models.BroadcastEvent

		err = msgpack.DecodeMsgPack(t.Payload(), &broadcastEvent)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1001, err: %s", err.Error()), delay: defaultDelay}
		}

		project, err := projectRepo.FetchProjectByID(ctx, broadcastEvent.ProjectID)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1002, err: %s", err.Error()), delay: 10 * time.Second}
		}

		var isDuplicate bool
		if len(broadcastEvent.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(ctx, broadcastEvent.ProjectID, broadcastEvent.IdempotencyKey)
			if err != nil {
				return &EndpointError{Err: fmt.Errorf("CODE: 1004, err: %s", err.Error()), delay: 10 * time.Second}
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

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subscriptions, true)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("failed to match subscriptions using filter, err: %s", err.Error()), delay: defaultDelay}
		}

		es, ss := getEndpointIDs(subscriptions)
		event.Endpoints = es

		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1005, err: %s", err.Error()), delay: 10 * time.Second}
		}

		q := eventQueue.(*redis.RedisQueue)
		ti, err := q.Inspector().GetTaskInfo(string(convoy.CreateEventQueue), broadcastEvent.JobID)
		if err != nil {
			log.WithError(err).Error("failed to get task from queue")
			return &EndpointError{Err: fmt.Errorf("failed to get task from queue"), delay: 10 * time.Second}
		}

		lastRunErrored := ti.LastErr == ErrFailedToWriteToQueue.Error()
		if event.IsDuplicateEvent && !lastRunErrored {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		err = writeEventDeliveriesToQueue(
			ctx, ss, event, project, eventDeliveryRepo,
			eventQueue, deviceRepo, endpointRepo,
		)

		if err != nil {
			log.WithError(err).Error(ErrFailedToWriteToQueue)
			return &EndpointError{Err: ErrFailedToWriteToQueue, delay: 10 * time.Second}
		}

		fmt.Println("HERE")
		return nil
	}
}

func getEndpointIDs(subs []datastore.Subscription) ([]string, []datastore.Subscription) {
	subMap := make(map[string]*datastore.Subscription)
	endpointIds := make([]string, 0, len(subs))

	var sub *datastore.Subscription
	for i := range subs {
		sub = &subs[i]
		if sub.Type == datastore.SubscriptionTypeAPI && !util.IsStringEmpty(sub.EndpointID) {
			if _, ok := subMap[sub.EndpointID]; !ok {
				subMap[sub.EndpointID] = sub
				endpointIds = append(endpointIds, sub.EndpointID)
			}
		}
	}

	subscriptionsIds := make([]datastore.Subscription, 0, len(subMap))
	for _, s := range subMap {
		subscriptionsIds = append(subscriptionsIds, *s)
	}

	return endpointIds, subscriptionsIds
}
