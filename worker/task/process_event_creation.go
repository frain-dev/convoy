package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessEventCreated(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {

		var event datastore.Event
		err := json.Unmarshal(t.Payload(), &event)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}
		event.DocumentStatus = datastore.ActiveDocumentStatus

		var group *datastore.Group
		var subscriptions []datastore.Subscription

		groupCacheKey := convoy.GroupsCacheKey.Get(event.GroupID).String()
		err = cache.Get(ctx, groupCacheKey, &group)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		if group == nil {
			group, err = groupRepo.FetchGroupByID(ctx, event.GroupID)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			err = cache.Set(ctx, groupCacheKey, group, 10*time.Minute)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}
		}

		if group.Type == datastore.OutgoingGroup {
			var app *datastore.Application

			appCacheKey := convoy.ApplicationsCacheKey.Get(event.AppID).String()
			err = cache.Get(ctx, appCacheKey, &app)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			// cache miss, load from db
			if app == nil {
				app, err = appRepo.FindApplicationByID(ctx, event.AppID)
				if err != nil {
					return &EndpointError{Err: err, delay: 10 * time.Second}
				}

				err = cache.Set(ctx, appCacheKey, app, 10*time.Minute)
				if err != nil {
					return &EndpointError{Err: err, delay: 10 * time.Second}
				}
			}

			subs, err := subRepo.FindSubscriptionsByAppID(ctx, group.UID, app.UID)
			if err != nil {
				return &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: 10 * time.Second}
			}

			subscriptions = matchSubscriptions(string(event.EventType), subs)
		} else if group.Type == datastore.IncomingGroup {
			subscriptions, err = subRepo.FindSubscriptionsBySourceIDs(ctx, group.UID, event.SourceID)
			if err != nil {
				log.Errorf("error fetching subscriptions for this source %s", err)
				return &EndpointError{Err: errors.New("error fetching subscriptions for this source"), delay: 10 * time.Second}
			}
		}

		event.MatchedEndpoints = len(subscriptions)
		err = eventRepo.CreateEvent(ctx, &event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		intervalSeconds := group.Config.Strategy.Duration
		retryLimit := group.Config.Strategy.RetryCount

		for _, s := range subscriptions {
			app, err := appRepo.FindApplicationByID(ctx, s.AppID)
			if err != nil {
				log.Errorf("Error fetching applcation %s", err)
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			endpoint, err := appRepo.FindApplicationEndpointByID(ctx, app.UID, s.EndpointID)
			if err != nil {
				log.Errorf("Error fetching endpoint %s", err)
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			s.Endpoint = endpoint

			metadata := &datastore.Metadata{
				NumTrials:       0,
				RetryLimit:      retryLimit,
				Data:            event.Data,
				IntervalSeconds: intervalSeconds,
				Strategy:        group.Config.Strategy.Type,
				NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
			}

			eventDelivery := &datastore.EventDelivery{UID: uuid.New().String(),
				SubscriptionID:   s.UID,
				AppID:            app.UID,
				Metadata:         metadata,
				GroupID:          group.UID,
				EventID:          event.UID,
				EndpointID:       s.EndpointID,
				DeviceID:         s.DeviceID,
				ForwardedHeaders: event.ForwardedHeaders,

				Status:           getEventDeliveryStatus(&s, app),
				DeliveryAttempts: []datastore.DeliveryAttempt{},
				DocumentStatus:   datastore.ActiveDocumentStatus,
				CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			}

			err = eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
			if err != nil {
				log.WithError(err).Error("error occurred creating event delivery")
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			taskName := convoy.EventProcessor

			// This event delivery will be picked up by the convoy stream command(if it is currently running).
			// Otherwise, it will be lost to the wind? workaround for this is to disable the subscriptions created
			// while the command was running, however in that scenario the event deliveries will still be created,
			// but will be in the datastore.DiscardedEventStatus status, we can also delete the subscription, that is a firmer solution
			if eventDelivery.Status != datastore.DiscardedEventStatus && s.Type != datastore.SubscriptionTypeCLI {
				payload := json.RawMessage(eventDelivery.UID)

				job := &queue.Job{
					ID:      eventDelivery.UID,
					Payload: payload,
					Delay:   1 * time.Second,
				}
				err = eventQueue.Write(taskName, convoy.EventQueue, job)
				if err != nil {
					log.Errorf("Error occurred sending new event to the queue %s", err)
				}
			}
		}

		return nil
	}
}

func matchSubscriptions(eventType string, subscriptions []datastore.Subscription) []datastore.Subscription {
	var matched []datastore.Subscription
	for _, sub := range subscriptions {
		for _, ev := range sub.FilterConfig.EventTypes {
			if ev == eventType || ev == "*" { // if this event type matches, or is *, add the subscription to matched
				matched = append(matched, sub)
			}
		}
	}

	return matched
}

func getEventDeliveryStatus(subscription *datastore.Subscription, app *datastore.Application) datastore.EventDeliveryStatus {
	if app.IsDisabled || subscription.Status != datastore.ActiveSubscriptionStatus {
		return datastore.DiscardedEventStatus
	}

	return datastore.ScheduledEventStatus
}
