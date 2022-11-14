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

func ProcessEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, search searcher.Searcher, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
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

		subscriptions, err = findSubscriptions(ctx, endpointRepo, cache, subRepo, group, &event)
		if err != nil {
			return err
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

			if s.Type == datastore.SubscriptionTypeAPI {
				endpoint, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID)
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
				Metadata:       metadata,
				GroupID:        group.UID,
				EventID:        event.UID,
				EndpointID:     s.EndpointID,
				DeviceID:       s.DeviceID,
				Headers:        headers,

				Status:           getEventDeliveryStatus(ctx, &s, s.Endpoint, deviceRepo),
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

func findSubscriptions(ctx context.Context, endpointRepo datastore.EndpointRepository, cache cache.Cache, subRepo datastore.SubscriptionRepository, group *datastore.Group, event *datastore.Event) ([]datastore.Subscription, error) {
	var subscriptions []datastore.Subscription
	var err error

	if group.Type == datastore.OutgoingGroup {
		for _, endpointID := range event.Endpoints {
			var endpoint *datastore.Endpoint

			endpointCacheKey := convoy.EndpointsCacheKey.Get(endpointID).String()
			err = cache.Get(ctx, endpointCacheKey, &endpoint)
			if err != nil {
				return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
			}

			// cache miss, load from db
			if endpoint == nil {
				endpoint, err = endpointRepo.FindEndpointByID(ctx, endpointID)
				if err != nil {
					return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
				}

				err = cache.Set(ctx, endpointCacheKey, endpoint, 10*time.Minute)
				if err != nil {
					return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
				}
			}

			subs, err := subRepo.FindSubscriptionsByEndpointID(ctx, group.UID, endpoint.UID)
			if err != nil {
				return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: 10 * time.Second}
			}

			subs = matchSubscriptions(string(event.EventType), subs)
			subscriptions = append(subscriptions, subs...)

		}
	} else if group.Type == datastore.IncomingGroup {
		subscriptions, err = subRepo.FindSubscriptionsBySourceIDs(ctx, group.UID, event.SourceID)
		if err != nil {
			log.Errorf("error fetching subscriptions for this source %s", err)
			return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for this source"), delay: 10 * time.Second}
		}
	}

	return subscriptions, nil
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

func getEventDeliveryStatus(ctx context.Context, subscription *datastore.Subscription, endpoint *datastore.Endpoint, deviceRepo datastore.DeviceRepository) datastore.EventDeliveryStatus {
	if endpoint != nil && endpoint.IsDisabled {
		return datastore.DiscardedEventStatus
	}

	if subscription.Status != datastore.ActiveSubscriptionStatus {
		return datastore.DiscardedEventStatus
	} else {
		if !util.IsStringEmpty(subscription.DeviceID) {
			device, err := deviceRepo.FetchDeviceByID(ctx, subscription.DeviceID, endpoint.UID, endpoint.GroupID)
			if err != nil {
				log.WithError(err).Error("an error occurred fetching the subcriptions's device")
				return datastore.DiscardedEventStatus
			}

			if device.Status != datastore.DeviceStatusOnline {
				return datastore.DiscardedEventStatus
			}
		}
	}

	return datastore.ScheduledEventStatus
}
