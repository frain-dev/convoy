package task

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/disq"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessEventCreation(
	appRepo datastore.ApplicationRepository,
	eventRepo datastore.EventRepository,
	groupRepo datastore.GroupRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	cache cache.Cache,
	eventQueue queue.Queuer,
	subRepo datastore.SubscriptionRepository,
) func(job *queue.Job) error {
	return func(job *queue.Job) error {
		event := job.Event
		ctx := context.Background()

		var group *datastore.Group
		groupCacheKey := convoy.GroupsCacheKey.Get(event.GroupID).String()
		err := cache.Get(ctx, groupCacheKey, &group)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		if group == nil {
			group, err = groupRepo.FetchGroupByID(ctx, event.GroupID)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
			}

			err = cache.Set(ctx, groupCacheKey, group, 10*time.Minute)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
			}
		}

		var subscriptions []datastore.Subscription

		if group.Type == datastore.OutgoingGroup {
			var app *datastore.Application

			appCacheKey := convoy.ApplicationsCacheKey.Get(event.AppID).String()
			err = cache.Get(ctx, appCacheKey, &app)
			if err != nil {
				return &disq.Error{Err: errors.New("cache error"), Delay: 10 * time.Second}
			}

			// cache miss, load from db
			if app == nil {
				app, err = appRepo.FindApplicationByID(ctx, event.AppID)
				if err != nil {
					return &disq.Error{Err: err, Delay: 10 * time.Second}
				}

				err = cache.Set(ctx, appCacheKey, app, 10*time.Minute)
				if err != nil {
					return &disq.Error{Err: err, Delay: 10 * time.Second}
				}
			}

			subscriptions, err = subRepo.FindSubscriptionByEventType(ctx, group.UID, app.UID, event.EventType)
			if err != nil {
				return &disq.Error{Err: errors.New("error fetching subscriptions for event type"), Delay: 10 * time.Second}
			}
		} else if group.Type == datastore.IncomingGroup {
			subscriptions, err = subRepo.FindSubscriptionBySourceIDs(ctx, group.UID, event.SourceID)
			if err != nil {
				return &disq.Error{Err: errors.New("error fetching subscriptions for this source"), Delay: 10 * time.Second}
			}
		}

		event.MatchedEndpoints = len(subscriptions)
		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		intervalSeconds := group.Config.Strategy.Duration
		retryLimit := group.Config.Strategy.RetryCount

		for _, s := range subscriptions {
			app, err := appRepo.FindApplicationByID(ctx, s.AppID)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
			}

			endpoint, err := appRepo.FindApplicationEndpointByID(ctx, app.UID, s.EndpointID)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
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

			eventDelivery := &datastore.EventDelivery{
				UID:            uuid.New().String(),
				SubscriptionID: s.UID,
				AppID:          app.UID,
				Metadata:       metadata,
				GroupID:        group.UID,
				EventID:        event.UID,
				EndpointID:     s.EndpointID,

				Status:           getEventDeliveryStatus(s, app),
				DeliveryAttempts: []datastore.DeliveryAttempt{},
				DocumentStatus:   datastore.ActiveDocumentStatus,
				CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			}

			err = eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
			if err != nil {
				log.WithError(err).Error("error occurred creating event delivery")
			}

			taskName := convoy.EventProcessor.SetPrefix(group.Name)
			if eventDelivery.Status != datastore.DiscardedEventStatus {
				job := &queue.Job{
					ID: eventDelivery.UID,
				}

				err = eventQueue.Publish(ctx, taskName, job, 1*time.Second)
				if err != nil {
					log.Errorf("Error occurred sending new event to the queue %s", err)
				}
			}
		}

		return nil
	}
}

func getEventDeliveryStatus(subscription datastore.Subscription, app *datastore.Application) datastore.EventDeliveryStatus {
	if app.IsDisabled || subscription.Status != datastore.ActiveSubscriptionStatus {
		return datastore.DiscardedEventStatus
	}

	return datastore.ScheduledEventStatus
}
