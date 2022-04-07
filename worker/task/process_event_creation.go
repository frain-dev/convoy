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
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessEventCreated(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer) func(job *queue.Job) error {
	return func(job *queue.Job) error {
		event := job.Event
		ctx := context.Background()

		var app *datastore.Application
		appCacheKey := convoy.ApplicationsCacheKey.Get(event.AppMetadata.UID).String()

		// fetch from cache
		err := cache.Get(ctx, appCacheKey, &app)
		if err != nil {
			return &EndpointError{Err: errors.New("cache error"), delay: time.Minute}
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
				return errors.New(msg)
			}

			err = cache.Set(ctx, appCacheKey, app, time.Minute)
			if err != nil {
				return err
			}
		}

		g, err := groupRepo.FetchGroupByID(ctx, event.AppMetadata.GroupID)
		if err != nil {
			return &EndpointError{Err: err, delay: time.Minute}
		}

		matchedEndpoints := matchEndpointsForDelivery(event.EventType, app.Endpoints, nil)
		event.MatchedEndpoints = len(matchedEndpoints)
		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		var intervalSeconds uint64
		var retryLimit uint64
		if string(g.Config.Strategy.Type) == string(config.DefaultStrategyProvider) {
			intervalSeconds = g.Config.Strategy.Default.IntervalSeconds
			retryLimit = g.Config.Strategy.Default.RetryLimit
		} else if string(g.Config.Strategy.Type) == string(config.ExponentialBackoffStrategyProvider) {
			intervalSeconds = 0
			retryLimit = g.Config.Strategy.ExponentialBackoff.RetryLimit
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
					Strategy:        g.Config.Strategy.Type,
					NumTrials:       0,
					IntervalSeconds: intervalSeconds,
					RetryLimit:      retryLimit,
					NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
				},
				Status:           getEventDeliveryStatus(v),
				DeliveryAttempts: []datastore.DeliveryAttempt{},
				DocumentStatus:   datastore.ActiveDocumentStatus,
				CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			}

			err = eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
			if err != nil {
				log.WithError(err).Error("error occurred creating event delivery")
			}

			taskName := convoy.EventProcessor.SetPrefix(g.Name)
			if eventDelivery.Status != datastore.DiscardedEventStatus {
				err = eventQueue.WriteEventDelivery(ctx, taskName, eventDelivery, 1*time.Second)
				if err != nil {
					log.Errorf("Error occurred sending new event to the queue %s", err)
				}
			}
		}

		return nil
	}
}

func getEventDeliveryStatus(endpoint datastore.Endpoint) datastore.EventDeliveryStatus {
	if endpoint.Status != datastore.ActiveEndpointStatus {
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
