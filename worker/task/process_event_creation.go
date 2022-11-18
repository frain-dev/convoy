package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessEventCreation(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, search searcher.Searcher, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
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

			subscriptions, err = matchSubscriptionsUsingFilter(ctx, event.Data, subRepo, subs)
			if err != nil {
				return &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: 10 * time.Second}
			}
		} else if group.Type == datastore.IncomingGroup {
			subs, err := subRepo.FindSubscriptionsBySourceIDs(ctx, group.UID, event.SourceID)
			if err != nil {
				log.Errorf("error fetching subscriptions for this source %s", err)
				return &EndpointError{Err: errors.New("error fetching subscriptions for this source"), delay: 10 * time.Second}
			}

			subscriptions, err = matchSubscriptionsUsingFilter(ctx, event.Data, subRepo, subs)
			if err != nil {
				return &EndpointError{Err: errors.New("error fetching subscriptions for this source"), delay: 10 * time.Second}
			}
		}

		event.MatchedEndpoints = len(subscriptions)
		err = eventRepo.CreateEvent(ctx, &event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		ec := &EventDeliveryConfig{group: group}

		for _, s := range subscriptions {
			ec.subscription = &s
			headers := event.Headers
			app, err := appRepo.FindApplicationByID(ctx, s.AppID)
			if err != nil {
				log.Errorf("Error fetching applcation %s", err)
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			if s.Type == datastore.SubscriptionTypeAPI {
				endpoint, err := appRepo.FindApplicationEndpointByID(ctx, app.UID, s.EndpointID)
				if err != nil {
					log.Errorf("Error fetching endpoint %s", err)
					return &EndpointError{Err: err, delay: 10 * time.Second}
				}

				if endpoint.Authentication != nil && endpoint.Authentication.Type == datastore.APIKeyAuthentication {
					headers = make(httpheader.HTTPHeader)
					headers[endpoint.Authentication.ApiKey.HeaderName] = []string{endpoint.Authentication.ApiKey.HeaderValue}
					headers.MergeHeaders(event.Headers)
				}

				s.Endpoint = endpoint
			}

			rc, err := ec.retryConfig()
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}

			}

			metadata := &datastore.Metadata{
				NumTrials:       0,
				RetryLimit:      rc.RetryCount,
				Data:            event.Data,
				IntervalSeconds: rc.Duration,
				Strategy:        rc.Type,
				NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
			}

			eventDelivery := &datastore.EventDelivery{UID: uuid.New().String(),
				SubscriptionID: s.UID,
				AppID:          app.UID,
				Metadata:       metadata,
				GroupID:        group.UID,
				EventID:        event.UID,
				EndpointID:     s.EndpointID,
				DeviceID:       s.DeviceID,
				Headers:        headers,

				Status:           getEventDeliveryStatus(ctx, &s, app, deviceRepo),
				DeliveryAttempts: []datastore.DeliveryAttempt{},
				DocumentStatus:   datastore.ActiveDocumentStatus,
				CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			}

			if s.Type == datastore.SubscriptionTypeCLI {
				eventDelivery.CLIMetadata = &datastore.CLIMetadata{EventType: string(event.EventType)}
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
					log.Errorf("[asynq]: an error occurred sending event delivery to be dispatched %s", err)
				}
			}
		}

		job := &queue.Job{
			ID:      event.UID,
			Payload: t.Payload(), // t.Payload() is the original event bytes
			Delay:   5 * time.Second,
		}

		err = eventQueue.Write(convoy.IndexDocument, convoy.PriorityQueue, job)
		if err != nil {
			log.Errorf("[asynq]: an error occurred sending event to be indexed %s", err)
		}

		return nil
	}
}

func matchSubscriptionsUsingFilter(ctx context.Context, filter []byte, subRepo datastore.SubscriptionRepository, subscriptions []datastore.Subscription) ([]datastore.Subscription, error) {
	var matched []datastore.Subscription
	var payload map[string]interface{}
	err := json.Unmarshal(filter, &payload)
	if err != nil {
		return nil, err
	}

	for _, s := range subscriptions {
		isMatched, err := subRepo.TestSubscriptionFilter(ctx, payload, s.FilterConfig.Filter)
		if err != nil {
			return nil, err
		}

		if isMatched {
			matched = append(matched, s)
		}
	}

	return matched, nil
}

func getEventDeliveryStatus(ctx context.Context, subscription *datastore.Subscription, app *datastore.Application, deviceRepo datastore.DeviceRepository) datastore.EventDeliveryStatus {
	if app.IsDisabled {
		return datastore.DiscardedEventStatus
	}

	if subscription.Status != datastore.ActiveSubscriptionStatus {
		return datastore.DiscardedEventStatus
	} else {
		if !util.IsStringEmpty(subscription.DeviceID) {
			device, err := deviceRepo.FetchDeviceByID(ctx, subscription.DeviceID, app.UID, app.GroupID)
			if err != nil {
				log.WithError(err).Error("an error occurred fetching the subscription's device")
				return datastore.DiscardedEventStatus
			}

			if device.Status != datastore.DeviceStatusOnline {
				return datastore.DiscardedEventStatus
			}
		}
	}

	return datastore.ScheduledEventStatus
}
