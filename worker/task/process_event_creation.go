package task

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/disq"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessEventCreated(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer) func(job *queue.Job) error {
	return func(job *queue.Job) error {
		event := job.Event
		ctx := context.Background()

		var group *datastore.Group
		var app *datastore.Application

		appCacheKey := convoy.ApplicationsCacheKey.Get(event.AppMetadata.UID).String()
		err := cache.Get(ctx, appCacheKey, &app)
		if err != nil {
			return &disq.Error{Err: errors.New("cache error"), Delay: 10 * time.Second}
		}

		// cache miss, load from db
		if app == nil {
			app, err = appRepo.FindApplicationByID(ctx, event.AppMetadata.UID)
			if err != nil {

				msg := "an error occurred while retrieving app details"

				if errors.Is(err, datastore.ErrApplicationNotFound) {
					msg = err.Error()
				}

				log.WithError(err).Error("failed to fetch app")
				return &disq.Error{Err: errors.New(msg), Delay: 10 * time.Second}
			}

			err = cache.Set(ctx, appCacheKey, app, 10*time.Minute)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
			}
		}

		groupCacheKey := convoy.GroupsCacheKey.Get(event.AppMetadata.GroupID).String()
		err = cache.Get(ctx, groupCacheKey, &group)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		if group == nil {
			group, err = groupRepo.FetchGroupByID(ctx, event.AppMetadata.GroupID)
			if err != nil {
				return &disq.Error{Err: err, Delay: 10 * time.Second}
			}
		}

		err = cache.Set(ctx, groupCacheKey, &group, 5*time.Minute)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		matchedEndpoints := matchEndpointsForDelivery(event.EventType, app.Endpoints, nil)
		event.MatchedEndpoints = len(matchedEndpoints)
		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &disq.Error{Err: err, Delay: 10 * time.Second}
		}

		var intervalSeconds uint64
		var retryLimit uint64
		if string(group.Config.Strategy.Type) == string(config.DefaultStrategyProvider) {
			intervalSeconds = group.Config.Strategy.Default.IntervalSeconds
			retryLimit = group.Config.Strategy.Default.RetryLimit
		} else if string(group.Config.Strategy.Type) == string(config.ExponentialBackoffStrategyProvider) {
			intervalSeconds = 0
			retryLimit = group.Config.Strategy.Exponential.RetryLimit
		} else {
			return nil
		}

		for _, v := range matchedEndpoints {
			eventDelivery := &datastore.EventDelivery{
				UID: uuid.New().String(),
				EventMetadata: &datastore.EventMetadata{
					UID:       event.UID,
					EventType: event.EventType,
				},
				EndpointMetadata: &datastore.EndpointMetadata{
					UID:               v.UID,
					TargetURL:         v.TargetURL,
					Status:            v.Status,
					Secret:            v.Secret,
					Sent:              false,
					RateLimit:         v.RateLimit,
					RateLimitDuration: v.RateLimitDuration,
					HttpTimeout:       v.HttpTimeout,
				},
				AppMetadata: &datastore.AppMetadata{
					UID:          app.UID,
					Title:        app.Title,
					GroupID:      app.GroupID,
					SupportEmail: app.SupportEmail,
				},
				Metadata: &datastore.Metadata{
					Data:            event.Data,
					Strategy:        group.Config.Strategy.Type,
					NumTrials:       0,
					IntervalSeconds: intervalSeconds,
					RetryLimit:      retryLimit,
					NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
				},
				Status:           getEventDeliveryStatus(v, app),
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

func getEventDeliveryStatus(endpoint datastore.Endpoint, app *datastore.Application) datastore.EventDeliveryStatus {
	if app.IsDisabled || endpoint.Status != datastore.ActiveEndpointStatus {
		return datastore.DiscardedEventStatus
	}

	return datastore.ScheduledEventStatus
}

func matchEndpointsForDelivery(ev datastore.EventType, endpoints, matched []datastore.Endpoint) []datastore.Endpoint {
	if len(endpoints) == 0 {
		return matched
	}

	if matched == nil {
		matched = make([]datastore.Endpoint, 0)
	}

	e := endpoints[0]
	for _, v := range e.Events {
		if v == string(ev) || v == "*" {
			matched = append(matched, e)
			break
		}
	}

	return matchEndpointsForDelivery(ev, endpoints[1:], matched)
}
