package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

type CreateEvent struct {
	Event              datastore.Event
	CreateSubscription bool
}

func ProcessEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var createEvent CreateEvent
		var event datastore.Event

		err := json.Unmarshal(t.Payload(), &createEvent)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		var project *datastore.Project
		var subscriptions []datastore.Subscription

		event = createEvent.Event

		projectCacheKey := convoy.ProjectsCacheKey.Get(event.ProjectID).String()
		err = cache.Get(ctx, projectCacheKey, &project)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		if project == nil {
			project, err = projectRepo.FetchProjectByID(ctx, event.ProjectID)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			err = cache.Set(ctx, projectCacheKey, project, 10*time.Minute)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}
		}

		subscriptions, err = findSubscriptions(ctx, endpointRepo, cache, subRepo, project, &createEvent)
		if err != nil {
			return err
		}

		_, err = eventRepo.FindEventByID(ctx, project.UID, event.UID)
		if err != nil {
			if len(event.Endpoints) < 1 {
				var endpointIDs []string
				for _, s := range subscriptions {
					if s.Type != datastore.SubscriptionTypeCLI {
						endpointIDs = append(endpointIDs, s.EndpointID)
					}
				}
				event.Endpoints = endpointIDs
			}

			err = eventRepo.CreateEvent(ctx, &event)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}
		}

		event.MatchedEndpoints = len(subscriptions)
		ec := &EventDeliveryConfig{project: project}

		for _, s := range subscriptions {
			ec.subscription = &s
			headers := event.Headers

			if s.Type == datastore.SubscriptionTypeAPI {
				endpoint, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, project.UID)
				if err != nil {
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
				Raw:             event.Raw,
				IntervalSeconds: rc.Duration,
				Strategy:        rc.Type,
				NextSendTime:    time.Now(),
			}

			eventDelivery := &datastore.EventDelivery{
				UID:            ulid.Make().String(),
				SubscriptionID: s.UID,
				Metadata:       metadata,
				ProjectID:      project.UID,
				EventID:        event.UID,
				EndpointID:     s.EndpointID,
				DeviceID:       s.DeviceID,
				Headers:        headers,

				Status:           getEventDeliveryStatus(ctx, &s, s.Endpoint, deviceRepo),
				DeliveryAttempts: []datastore.DeliveryAttempt{},
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			if s.Type == datastore.SubscriptionTypeCLI {
				event.Endpoints = []string{}
				eventDelivery.CLIMetadata = &datastore.CLIMetadata{
					EventType: string(event.EventType),
					SourceID:  event.SourceID,
				}
			}

			err = eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			if eventDelivery.Status != datastore.DiscardedEventStatus {
				payload := EventDelivery{
					EventDeliveryID: eventDelivery.UID,
					ProjectID:       project.UID,
				}

				data, err := json.Marshal(payload)
				if err != nil {
					return &EndpointError{Err: err, delay: 10 * time.Second}
				}

				job := &queue.Job{
					ID:      eventDelivery.UID,
					Payload: data,
					Delay:   1 * time.Second,
				}

				if s.Type == datastore.SubscriptionTypeAPI {
					err = eventQueue.Write(convoy.EventProcessor, convoy.EventQueue, job)
					if err != nil {
						log.FromContext(ctx).WithError(err).Errorf("[asynq]: an error occurred sending event delivery to be dispatched")
					}
				} else if s.Type == datastore.SubscriptionTypeCLI {
					err = eventQueue.Write(convoy.StreamCliEventsProcessor, convoy.StreamQueue, job)
					if err != nil {
						log.FromContext(ctx).WithError(err).Error("[asynq]: an error occurred sending event delivery to the stream queue")
					}
				}
			}
		}

		eBytes, err := json.Marshal(event)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("[asynq]: an error occurred marshalling event to be indexed")
		}

		job := &queue.Job{
			ID:      event.UID,
			Payload: eBytes,
			Delay:   5 * time.Second,
		}

		err = eventQueue.Write(convoy.IndexDocument, convoy.SearchIndexQueue, job)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("[asynq]: an error occurred sending event to be indexed")
		}

		return nil
	}
}

func findSubscriptions(ctx context.Context, endpointRepo datastore.EndpointRepository, cache cache.Cache, subRepo datastore.SubscriptionRepository, project *datastore.Project, createEvent *CreateEvent) ([]datastore.Subscription, error) {
	var subscriptions []datastore.Subscription
	var err error

	event := createEvent.Event
	if project.Type == datastore.OutgoingProject {
		for _, endpointID := range event.Endpoints {
			var endpoint *datastore.Endpoint

			endpointCacheKey := convoy.EndpointsCacheKey.Get(endpointID).String()
			err = cache.Get(ctx, endpointCacheKey, &endpoint)
			if err != nil {
				return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
			}

			// cache miss, load from db
			if endpoint == nil {
				endpoint, err = endpointRepo.FindEndpointByID(ctx, endpointID, project.UID)
				if err != nil {
					return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
				}

				err = cache.Set(ctx, endpointCacheKey, endpoint, 10*time.Minute)
				if err != nil {
					return subscriptions, &EndpointError{Err: err, delay: 10 * time.Second}
				}
			}

			subs, err := subRepo.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
			if err != nil {
				return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: 10 * time.Second}
			}

			if len(subs) == 0 && createEvent.CreateSubscription {
				subs := generateSubscription(project, endpoint)
				err := subRepo.CreateSubscription(ctx, project.UID, subs)
				if err != nil {
					return subscriptions, &EndpointError{Err: errors.New("error creating subscription for endpoint"), delay: 10 * time.Second}
				}

				subscriptions = append(subscriptions, *subs)
				return subscriptions, nil
			}

			subs = matchSubscriptions(string(event.EventType), subs)

			subs, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subs)
			if err != nil {
				return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: 10 * time.Second}
			}

			subscriptions = append(subscriptions, subs...)
		}
	} else if project.Type == datastore.IncomingProject {
		subs, err := subRepo.FindSubscriptionsBySourceID(ctx, project.UID, event.SourceID)
		if err != nil {
			return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for this source"), delay: 10 * time.Second}
		}

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subs)
		if err != nil {
			log.WithError(err).Error("error find a matching subscription for this source")
			return subscriptions, &EndpointError{Err: errors.New("error find a matching subscription for this source"), delay: 10 * time.Second}
		}
	}

	return subscriptions, nil
}

func matchSubscriptionsUsingFilter(ctx context.Context, e datastore.Event, subRepo datastore.SubscriptionRepository, subscriptions []datastore.Subscription) ([]datastore.Subscription, error) {
	var matched []datastore.Subscription
	var payload interface{}
	err := json.Unmarshal(e.Data, &payload)
	if err != nil {
		return nil, err
	}

	for _, s := range subscriptions {
		isBodyMatched, err := subRepo.TestSubscriptionFilter(ctx, payload, s.FilterConfig.Filter.Body.Map())
		if err != nil {
			return nil, err
		}

		isHeaderMatched, err := subRepo.TestSubscriptionFilter(ctx, e.GetRawHeaders(), s.FilterConfig.Filter.Headers.Map())
		if err != nil {
			return nil, err
		}

		// true & true => true
		// true & false => false
		// false & false => false
		// false & true => false
		isMatched := isHeaderMatched && isBodyMatched

		if isMatched {
			matched = append(matched, s)
		}
	}

	return matched, nil
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
	switch subscription.Type {
	case datastore.SubscriptionTypeAPI:
		if endpoint.Status != datastore.ActiveEndpointStatus {
			return datastore.DiscardedEventStatus
		}
	case datastore.SubscriptionTypeCLI:
		device, err := deviceRepo.FetchDeviceByID(ctx, subscription.DeviceID, "", subscription.ProjectID)
		if err != nil {
			return datastore.DiscardedEventStatus
		}

		if device.Status != datastore.DeviceStatusOnline {
			return datastore.DiscardedEventStatus
		}
	default:
		log.FromContext(ctx).Debug("unknown subscription type: %s", subscription.Type)
	}

	return datastore.ScheduledEventStatus
}

func generateSubscription(project *datastore.Project, endpoint *datastore.Endpoint) *datastore.Subscription {
	return &datastore.Subscription{
		ProjectID:  project.UID,
		UID:        ulid.Make().String(),
		Name:       fmt.Sprintf("%s-subscription", endpoint.Title),
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
