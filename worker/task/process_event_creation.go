package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/transform"

	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

type CreateEventTaskParams struct {
	UID            string
	ProjectID      string
	OwnerID        string            `json:"owner_id"`
	AppID          string            `json:"app_id"`
	EndpointID     string            `json:"endpoint_id"`
	SourceID       string            `json:"source_id"`
	Data           json.RawMessage   `json:"data"`
	EventType      string            `json:"event_type"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	IdempotencyKey string            `json:"idempotency_key"`
}

type CreateEvent struct {
	Params             CreateEventTaskParams
	Event              *datastore.Event
	CreateSubscription bool
}

func ProcessEventCreation(
	endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer,
	subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var createEvent CreateEvent
		var event *datastore.Event
		var projectID string

		err := msgpack.DecodeMsgPack(t.Payload(), &createEvent)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &createEvent)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		if createEvent.Event != nil {
			projectID = createEvent.Event.ProjectID
		} else {
			projectID = createEvent.Params.ProjectID
		}

		project, err := projectRepo.FetchProjectByID(ctx, projectID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		if createEvent.Event == nil {
			event, err = buildEvent(ctx, eventRepo, endpointRepo, &createEvent.Params, project)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		} else {
			event = createEvent.Event
		}

		subscriptions, err := findSubscriptions(ctx, endpointRepo, subRepo, project, event, createEvent.CreateSubscription)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
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

			err = eventRepo.CreateEvent(ctx, event)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		if event.IsDuplicateEvent {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		return writeEventDeliveriesToQueue(
			ctx, subscriptions, event, project, eventDeliveryRepo,
			eventQueue, deviceRepo, endpointRepo,
		)
	}
}

func writeEventDeliveriesToQueue(ctx context.Context, subscriptions []datastore.Subscription, event *datastore.Event, project *datastore.Project, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, deviceRepo datastore.DeviceRepository, endpointRepo datastore.EndpointRepository) error {
	ec := &EventDeliveryConfig{project: project}
	for _, s := range subscriptions {
		ec.subscription = &s
		headers := event.Headers

		if s.Type == datastore.SubscriptionTypeAPI {
			endpoint, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, project.UID)
			if err != nil {
				return &EndpointError{Err: fmt.Errorf("CODE: 1006, err: %s", err.Error()), delay: defaultDelay}
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
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		raw := event.Raw
		data := event.Data

		if s.Function.Ptr() != nil && !util.IsStringEmpty(s.Function.String) {
			var payload map[string]interface{}
			err = json.Unmarshal(event.Data, &payload)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			transformer := transform.NewTransformer()
			mutated, _, err := transformer.Transform(s.Function.String, payload)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			bytes, err := json.Marshal(mutated)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			raw = string(bytes)
			data = bytes
		}

		metadata := &datastore.Metadata{
			Raw:             raw,
			Data:            data,
			Strategy:        rc.Type,
			NextSendTime:    time.Now(),
			IntervalSeconds: rc.Duration,
			RetryLimit:      rc.RetryCount,
		}

		eventDelivery := &datastore.EventDelivery{
			UID:              ulid.Make().String(),
			SubscriptionID:   s.UID,
			EventType:        event.EventType,
			Metadata:         metadata,
			ProjectID:        project.UID,
			EventID:          event.UID,
			EndpointID:       s.EndpointID,
			DeviceID:         s.DeviceID,
			Headers:          headers,
			IdempotencyKey:   event.IdempotencyKey,
			URLQueryParams:   event.URLQueryParams,
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
			return &EndpointError{Err: fmt.Errorf("CODE: 1008, err: %s", err.Error()), delay: defaultDelay}
		}

		if eventDelivery.Status != datastore.DiscardedEventStatus {
			payload := EventDelivery{
				EventDeliveryID: eventDelivery.UID,
				ProjectID:       eventDelivery.ProjectID,
			}

			data, err := msgpack.EncodeMsgPack(payload)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
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

	return nil
}

func findSubscriptions(ctx context.Context, endpointRepo datastore.EndpointRepository,
	subRepo datastore.SubscriptionRepository, project *datastore.Project, event *datastore.Event, shouldCreateSubscription bool,
) ([]datastore.Subscription, error) {
	var subscriptions []datastore.Subscription
	var err error

	if project.Type == datastore.OutgoingProject {
		for _, endpointID := range event.Endpoints {
			var endpoint *datastore.Endpoint

			endpoint, err = endpointRepo.FindEndpointByID(ctx, endpointID, project.UID)
			if err != nil {
				return subscriptions, &EndpointError{Err: err, delay: defaultDelay}
			}

			subs, err := subRepo.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
			if err != nil {
				return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: defaultDelay}
			}

			if len(subs) == 0 && shouldCreateSubscription {
				subs := generateSubscription(project, endpoint)
				err := subRepo.CreateSubscription(ctx, project.UID, subs)
				if err != nil {
					return subscriptions, &EndpointError{Err: errors.New("error creating subscription for endpoint"), delay: defaultDelay}
				}

				subscriptions = append(subscriptions, *subs)
				return subscriptions, nil
			}

			subs = matchSubscriptions(string(event.EventType), subs)

			subs, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subs, false)
			if err != nil {
				return subscriptions, &EndpointError{Err: errors.New("error fetching subscriptions for event type"), delay: defaultDelay}
			}

			subscriptions = append(subscriptions, subs...)
		}
	} else if project.Type == datastore.IncomingProject {
		subscriptions, err = subRepo.FindSubscriptionsBySourceID(ctx, project.UID, event.SourceID)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, subscriptions, false)
		if err != nil {
			log.WithError(err).Error("error find a matching subscription for this source")
			return subscriptions, &EndpointError{Err: errors.New("error find a matching subscription for this source"), delay: defaultDelay}
		}
	}

	return subscriptions, nil
}

func matchSubscriptionsUsingFilter(ctx context.Context, e *datastore.Event, subRepo datastore.SubscriptionRepository, subscriptions []datastore.Subscription, soft bool) ([]datastore.Subscription, error) {
	var matched []datastore.Subscription
	var payload interface{}
	err := json.Unmarshal(e.Data, &payload)
	if err != nil {
		return nil, err
	}

	for _, s := range subscriptions {
		isBodyMatched, err := subRepo.TestSubscriptionFilter(ctx, payload, s.FilterConfig.Filter.Body.Map())
		if err != nil && soft {
			log.WithError(err).Errorf("subcription (%s) failed to match body", s.UID)
			continue
		} else if err != nil {
			return nil, err
		}

		isHeaderMatched, err := subRepo.TestSubscriptionFilter(ctx, e.GetRawHeaders(), s.FilterConfig.Filter.Headers.Map())
		if err != nil && soft {
			log.WithError(err).Errorf("subscription (%s) failed to match header", s.UID)
			continue
		} else if err != nil {
			return nil, err
		}

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

func getEventDeliveryStatus(ctx context.Context, subscription *datastore.Subscription, endpoint *datastore.Endpoint,
	deviceRepo datastore.DeviceRepository,
) datastore.EventDeliveryStatus {
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
		Name:       fmt.Sprintf("%s-subscription", endpoint.Name),
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

func buildEvent(ctx context.Context, eventRepo datastore.EventRepository, endpointRepo datastore.EndpointRepository,
	eventParams *CreateEventTaskParams, project *datastore.Project,
) (*datastore.Event, error) {
	var isDuplicate bool
	if !util.IsStringEmpty(eventParams.IdempotencyKey) {
		events, err := eventRepo.FindEventsByIdempotencyKey(ctx, project.UID, eventParams.IdempotencyKey)
		if err != nil {
			return nil, err
		}

		isDuplicate = len(events) > 0
	}

	if project == nil {
		return nil, errors.New("an error occurred while creating event - invalid project")
	}

	if util.IsStringEmpty(eventParams.AppID) && util.IsStringEmpty(eventParams.EndpointID) && util.IsStringEmpty(eventParams.OwnerID) {
		return nil, errors.New("please provide an endpoint ID")
	}

	endpoints, err := findEndpoints(ctx, endpointRepo, eventParams, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoints")
		return nil, err
	}

	if len(endpoints) == 0 {
		return nil, errors.New("no valid endpoint found")
	}

	var endpointIDs []string
	for _, endpoint := range endpoints {
		endpointIDs = append(endpointIDs, endpoint.UID)
	}

	event := &datastore.Event{
		UID:              eventParams.UID,
		EventType:        datastore.EventType(eventParams.EventType),
		Data:             eventParams.Data,
		Raw:              string(eventParams.Data),
		IdempotencyKey:   eventParams.IdempotencyKey,
		IsDuplicateEvent: isDuplicate,
		Headers:          getCustomHeaders(eventParams.CustomHeaders),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Endpoints:        endpointIDs,
		SourceID:         eventParams.SourceID,
		ProjectID:        project.UID,
	}

	if (project.Config == nil || project.Config.Strategy == nil) ||
		(project.Config.Strategy != nil && project.Config.Strategy.Type != datastore.LinearStrategyProvider &&
			project.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, errors.New("retry strategy not defined in configuration")
	}

	return event, nil
}

func findEndpoints(ctx context.Context, endpointRepo datastore.EndpointRepository, newMessage *CreateEventTaskParams,
	project *datastore.Project,
) ([]datastore.Endpoint, error) {
	var endpoints []datastore.Endpoint

	if !util.IsStringEmpty(newMessage.EndpointID) {
		endpoint, err := endpointRepo.FindEndpointByID(ctx, newMessage.EndpointID, project.UID)
		if err != nil {
			return endpoints, err
		}

		endpoints = append(endpoints, *endpoint)
		return endpoints, nil
	}

	if !util.IsStringEmpty(newMessage.OwnerID) {
		endpoints, err := endpointRepo.FindEndpointsByOwnerID(ctx, project.UID, newMessage.OwnerID)
		if err != nil {
			return endpoints, err
		}

		return endpoints, nil
	}

	if !util.IsStringEmpty(newMessage.AppID) {
		_endpoints, err := endpointRepo.FindEndpointsByAppID(ctx, newMessage.AppID, project.UID)
		if err != nil {
			return _endpoints, err
		}

		return _endpoints, nil
	}

	return endpoints, nil
}
