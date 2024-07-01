package task

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/guregu/null.v4"
	"time"

	"github.com/frain-dev/convoy/queue/redis"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

var (
	ErrFailedToWriteToQueue = errors.New("failed to write to event delivery queue")
	defaultBroadcastDelay   = 30 * time.Second
)

func ProcessBroadcastEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository, subscriptionsTable memorystore.ITable) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) (err error) {
		var broadcastEvent models.BroadcastEvent

		err = msgpack.DecodeMsgPack(t.Payload(), &broadcastEvent)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1001, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}

		project, err := projectRepo.FetchProjectByID(ctx, broadcastEvent.ProjectID)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1002, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}

		var isDuplicate bool
		if len(broadcastEvent.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(ctx, broadcastEvent.ProjectID, broadcastEvent.IdempotencyKey)
			if err != nil {
				return &EndpointError{Err: fmt.Errorf("CODE: 1004, err: %s", err.Error()), delay: defaultBroadcastDelay}
			}

			isDuplicate = len(events) > 0
		}

		mKeys := memorystore.NewKey(project.UID, "*")
		matchAllSubs := getSubscriptionsFromRow(subscriptionsTable.Get(mKeys))

		key := memorystore.NewKey(project.UID, broadcastEvent.EventType)
		eventTypeSubs := getSubscriptionsFromRow(subscriptionsTable.Get(key))

		subscriptions := make([]datastore.Subscription, 0, len(matchAllSubs)+len(eventTypeSubs))
		subscriptions = append(subscriptions, eventTypeSubs...)
		subscriptions = append(subscriptions, matchAllSubs...)

		// subscriptions := joinSubscriptions(matchAllSubs, eventTypeSubs)

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
			AcknowledgedAt:   null.TimeFrom(time.Now()),
		}

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subscriptions, true)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("failed to match subscriptions using filter, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}

		es, ss := getEndpointIDs(subscriptions)
		event.Endpoints = es

		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1005, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}

		q := eventQueue.(*redis.RedisQueue)
		ti, err := q.Inspector().GetTaskInfo(string(convoy.CreateEventQueue), broadcastEvent.JobID)
		if err != nil {
			log.WithError(err).Error("failed to get task from queue")
			return &EndpointError{Err: fmt.Errorf("failed to get task from queue, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}

		lastRunErrored := ti.LastErr == ErrFailedToWriteToQueue.Error()
		if event.IsDuplicateEvent && !lastRunErrored {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		err = writeEventDeliveriesToQueue(ctx, ss, event, project, eventDeliveryRepo, eventQueue, deviceRepo, endpointRepo)
		if err != nil {
			log.WithError(err).Error(ErrFailedToWriteToQueue)
			return &EndpointError{Err: fmt.Errorf("%s, err: %s", ErrFailedToWriteToQueue.Error(), err.Error()), delay: defaultBroadcastDelay}
		}

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

func getSubscriptionsFromRow(row *memorystore.Row) []datastore.Subscription {
	if row == nil {
		return []datastore.Subscription{}
	}

	subs, ok := row.Value().([]datastore.Subscription)
	if !ok {
		return []datastore.Subscription{}
	}

	return subs
}

//func joinSubscriptions(sub1, sub2 []datastore.Subscription) []datastore.Subscription {
//	seen := make(map[string]bool)
//	result := []datastore.Subscription{}
//
//	// Iterate through the first slice and add unique subscriptions to the result
//	for _, sub := range sub1 {
//		if !seen[sub.UID] {
//			seen[sub.UID] = true
//			result = append(result, sub)
//		}
//	}
//
//	// Iterate through the second slice and add unique subscriptions to the result
//	for _, sub := range sub2 {
//		if !seen[sub.UID] {
//			seen[sub.UID] = true
//			result = append(result, sub)
//		}
//	}
//
//	return result
//}
//
