package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/guregu/null.v4"
	"strconv"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/flatten"

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
	AcknowledgedAt time.Time         `json:"acknowledged_at,omitempty"`
}

type CreateEvent struct {
	JobID              string `json:"jid" swaggerignore:"true"`
	Params             CreateEventTaskParams
	Event              *datastore.Event
	CreateSubscription bool
}

type DefaultEventChannel struct {
}

func NewDefaultEventChannel() *DefaultEventChannel {
	return &DefaultEventChannel{}
}

func (d *DefaultEventChannel) GetConfig() *EventChannelConfig {
	return &EventChannelConfig{
		Channel:      "default",
		DefaultDelay: defaultDelay,
	}
}

// create event
// find & match subscriptions & create deliveries (e = SUCCESS)
// deliver ed (ed = SUCCESS)
func (d *DefaultEventChannel) CreateEvent(ctx context.Context, t *asynq.Task, channel EventChannel, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, endpointRepo datastore.EndpointRepository, _ datastore.SubscriptionRepository, licenser license.Licenser) (*datastore.Event, error) {
	var createEvent CreateEvent
	var event *datastore.Event
	var projectID string

	err := msgpack.DecodeMsgPack(t.Payload(), &createEvent)
	if err != nil {
		err := json.Unmarshal(t.Payload(), &createEvent)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	if createEvent.Event != nil {
		projectID = createEvent.Event.ProjectID
	} else {
		projectID = createEvent.Params.ProjectID
	}

	project, err := projectRepo.FetchProjectByID(ctx, projectID)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	if createEvent.Event == nil {
		event, err = buildEvent(ctx, eventRepo, endpointRepo, &createEvent.Params, project)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	} else {
		event = createEvent.Event
	}

	_, err = eventRepo.FindEventByID(ctx, project.UID, event.UID)
	if err != nil { // 404
		err := updateEventMetadata(channel, event, createEvent.CreateSubscription)
		if err != nil {
			return nil, err
		}

		var isDuplicate bool
		if len(event.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(ctx, event.ProjectID, event.IdempotencyKey)
			if err != nil {
				return nil, &EndpointError{Err: err, delay: 10 * time.Second}
			}

			isDuplicate = len(events) > 0
		}
		event.IsDuplicateEvent = isDuplicate

		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	return event, nil
}

func updateEventMetadata(channel EventChannel, event *datastore.Event, createSubscription bool) error {
	metadata := make(map[string]string)
	metadata["channel"] = channel.GetConfig().Channel
	metadata["delay"] = strconv.FormatInt(int64(channel.GetConfig().DefaultDelay), 10)
	if createSubscription {
		metadata["createSubscription"] = "true"
	}
	m, err := json.Marshal(metadata)
	if err != nil {
		log.WithError(err).Error("failed to marshal metadata for event")
		return &EndpointError{Err: err, delay: defaultDelay}
	}
	event.Metadata = string(m)
	return err
}

func (d *DefaultEventChannel) MatchSubscriptions(ctx context.Context, metadata EventChannelMetadata, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, endpointRepo datastore.EndpointRepository, subRepo datastore.SubscriptionRepository, licenser license.Licenser) (*EventChannelSubResponse, error) {
	response := EventChannelSubResponse{}

	project, err := projectRepo.FetchProjectByID(ctx, metadata.Event.ProjectID)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}
	event, err := eventRepo.FindEventByID(ctx, project.UID, metadata.Event.UID)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	err = eventRepo.UpdateEventStatus(ctx, event, datastore.ProcessingStatus)
	if err != nil {
		return nil, err
	}

	var createSubscription bool
	if !util.IsStringEmpty(event.Metadata) {
		var m map[string]string
		err := json.Unmarshal([]byte(event.Metadata), &m)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
		cs := m["createSubscription"]
		createSubscription = !util.IsStringEmpty(cs) && cs == "true"
	}

	subscriptions, err := findSubscriptions(ctx, endpointRepo, subRepo, licenser, project, event, createSubscription)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	response.Event = event
	response.Project = project
	response.Subscriptions = subscriptions
	response.IsDuplicateEvent = event.IsDuplicateEvent

	return &response, nil
}

func ProcessEventCreation(ch *DefaultEventChannel, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository, licenser license.Licenser) func(context.Context, *asynq.Task) error {
	return ProcessEventCreationByChannel(ch, endpointRepo, eventRepo, projectRepo, eventQueue, subRepo, licenser)
}

func writeEventDeliveriesToQueue(ctx context.Context, subscriptions []datastore.Subscription, event *datastore.Event, project *datastore.Project, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, deviceRepo datastore.DeviceRepository, endpointRepo datastore.EndpointRepository, licenser license.Licenser) error {
	ec := &EventDeliveryConfig{project: project}

	eventDeliveries := make([]*datastore.EventDelivery, 0)
	for _, s := range subscriptions {
		ec.subscription = &s
		headers := event.Headers

		if s.Type == datastore.SubscriptionTypeAPI {
			endpoint, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, project.UID)
			if err != nil {
				if errors.Is(err, datastore.ErrEndpointNotFound) {
					continue
				}

				return &EndpointError{Err: fmt.Errorf("CODE: 1006, err: %s", err.Error()), delay: defaultDelay}
			}

			if endpoint.Authentication != nil && endpoint.Authentication.Type == datastore.APIKeyAuthentication {
				headers = make(httpheader.HTTPHeader)
				headers[endpoint.Authentication.ApiKey.HeaderName] = []string{endpoint.Authentication.ApiKey.HeaderValue}
				headers.MergeHeaders(event.Headers)
			}

			s.Endpoint = endpoint
		}

		rc, err := ec.RetryConfig()
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		raw := event.Raw
		data := event.Data

		if s.Function.Ptr() != nil && !util.IsStringEmpty(s.Function.String) && licenser.Transformations() {
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
			UID:            ulid.Make().String(),
			SubscriptionID: s.UID,
			EventType:      event.EventType,
			Metadata:       metadata,
			ProjectID:      project.UID,
			EventID:        event.UID,
			EndpointID:     s.EndpointID,
			DeviceID:       s.DeviceID,
			Headers:        headers,
			IdempotencyKey: event.IdempotencyKey,
			URLQueryParams: event.URLQueryParams,
			Status:         getEventDeliveryStatus(ctx, &s, s.Endpoint, deviceRepo),
			AcknowledgedAt: null.TimeFrom(time.Now()),
		}

		if s.Type == datastore.SubscriptionTypeCLI {
			event.Endpoints = []string{}
			eventDelivery.CLIMetadata = &datastore.CLIMetadata{
				EventType: string(event.EventType),
				SourceID:  event.SourceID,
			}
		}

		eventDeliveries = append(eventDeliveries, eventDelivery)
	}

	err := eventDeliveryRepo.CreateEventDeliveries(ctx, eventDeliveries)
	if err != nil {
		return &EndpointError{Err: fmt.Errorf("CODE: 1008, err: %s", err.Error()), delay: defaultDelay}
	}

	for i, eventDelivery := range eventDeliveries {
		s := subscriptions[i]
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
	subRepo datastore.SubscriptionRepository, licenser license.Licenser, project *datastore.Project, event *datastore.Event, shouldCreateSubscription bool,
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

			subs, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, licenser, subs, false)
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

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, licenser, subscriptions, false)
		if err != nil {
			log.WithError(err).Error("error find a matching subscription for this source")
			return subscriptions, &EndpointError{Err: errors.New("error find a matching subscription for this source"), delay: defaultDelay}
		}
	}

	return subscriptions, nil
}

func matchSubscriptionsUsingFilter(ctx context.Context, e *datastore.Event, subRepo datastore.SubscriptionRepository, licenser license.Licenser, subscriptions []datastore.Subscription, soft bool) ([]datastore.Subscription, error) {
	if !licenser.AdvancedSubscriptions() {
		return subscriptions, nil
	}

	var matched []datastore.Subscription

	// payload is interface{} and not map[string]interface{} because
	// map[string]interface{} won't work for array based json e.g:
	// [
	//	{
	//		"organization": "frain-dev"
	//	},
	//	{
	//		".members_url": "danvixent"
	//	}
	//]
	var payload interface{}
	err := json.Unmarshal(e.Data, &payload) // TODO(all): find a way to stop doing this repeatedly, json.Unmarshal is slow and costly
	if err != nil {
		return nil, err
	}

	flatPayload, err := flatten.Flatten(payload)
	if err != nil {
		return nil, err
	}

	headers := e.GetRawHeaders()
	var s *datastore.Subscription

	for i := range subscriptions {
		s = &subscriptions[i]
		if len(s.FilterConfig.Filter.Body) == 0 && len(s.FilterConfig.Filter.Headers) == 0 {
			matched = append(matched, *s)
			continue
		}

		isBodyMatched, err := subRepo.CompareFlattenedPayload(ctx, flatPayload, s.FilterConfig.Filter.Body, s.FilterConfig.Filter.IsFlattened)
		if err != nil && soft {
			log.WithError(err).Errorf("subcription (%s) failed to match body", s.UID)
			continue
		} else if err != nil {
			return nil, err
		}

		isHeaderMatched, err := subRepo.CompareFlattenedPayload(ctx, headers, s.FilterConfig.Filter.Headers, s.FilterConfig.Filter.IsFlattened)
		if err != nil && soft {
			log.WithError(err).Errorf("subscription (%s) failed to match header", s.UID)
			continue
		} else if err != nil {
			return nil, err
		}

		isMatched := isHeaderMatched && isBodyMatched

		if isMatched {
			matched = append(matched, *s)
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
		// TODO(all): we should discard events without endpoint id here.
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
		AcknowledgedAt:   null.TimeFrom(time.Now()),
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
