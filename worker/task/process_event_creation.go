package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
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
	UID            string            `json:"uid"`
	ProjectID      string            `json:"project_id"`
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
func (d *DefaultEventChannel) CreateEvent(ctx context.Context, t *asynq.Task, channel EventChannel, args EventChannelArgs) (*datastore.Event, error) {
	var createEvent CreateEvent
	var event *datastore.Event
	var projectID string

	// Start a new trace span for event creation
	startTime := time.Now()
	attributes := map[string]interface{}{
		"event.type": "event.creation",
		"channel":    channel,
	}

	err := msgpack.DecodeMsgPack(t.Payload(), &createEvent)
	if err != nil {
		err := json.Unmarshal(t.Payload(), &createEvent)
		if err != nil {
			args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	if createEvent.Event != nil {
		projectID = createEvent.Event.ProjectID
	} else {
		projectID = createEvent.Params.ProjectID
	}

	attributes["project.id"] = projectID

	project, err := args.projectRepo.FetchProjectByID(ctx, projectID)
	if err != nil {
		args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	if createEvent.Event == nil {
		event, err = buildEvent(ctx, args.eventRepo, args.endpointRepo, &createEvent.Params, project)
		if err != nil {
			args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	} else {
		event = createEvent.Event
	}

	attributes["event.id"] = event.UID

	_, err = args.eventRepo.FindEventByID(ctx, project.UID, event.UID)
	if err != nil { // 404
		err := updateEventMetadata(channel, event, createEvent.CreateSubscription)
		if err != nil {
			args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
			return nil, err
		}

		var isDuplicate bool
		if len(event.IdempotencyKey) > 0 {
			events, err := args.eventRepo.FindEventsByIdempotencyKey(ctx, event.ProjectID, event.IdempotencyKey)
			if err != nil {
				args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
				return nil, &EndpointError{Err: err, delay: 10 * time.Second}
			}

			isDuplicate = len(events) > 0
		}
		event.IsDuplicateEvent = isDuplicate

		err = args.eventRepo.CreateEvent(ctx, event)
		if err != nil {
			args.tracerBackend.Capture(ctx, "event.creation.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	args.tracerBackend.Capture(ctx, "event.creation.success", attributes, startTime, time.Now())
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

func (d *DefaultEventChannel) MatchSubscriptions(ctx context.Context, metadata EventChannelMetadata, args EventChannelArgs) (*EventChannelSubResponse, error) {
	response := EventChannelSubResponse{}

	project, err := args.projectRepo.FetchProjectByID(ctx, metadata.Event.ProjectID)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}
	event, err := args.eventRepo.FindEventByID(ctx, project.UID, metadata.Event.UID)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	err = args.eventRepo.UpdateEventStatus(ctx, event, datastore.ProcessingStatus)
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

	subscriptions, err := findSubscriptions(ctx, args.endpointRepo, args.subRepo, args.filterRepo, args.licenser, project, event, createSubscription)
	if err != nil {
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	response.Event = event
	response.Project = project
	response.Subscriptions = subscriptions
	response.IsDuplicateEvent = event.IsDuplicateEvent

	return &response, nil
}

func ProcessEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository, licenser license.Licenser, tracerBackend tracer.Backend) func(context.Context, *asynq.Task) error {
	ch := &DefaultEventChannel{}

	return ProcessEventCreationByChannel(ch, endpointRepo, eventRepo, projectRepo, eventQueue, subRepo, filterRepo, licenser, tracerBackend)
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
	subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository, licenser license.Licenser, project *datastore.Project, event *datastore.Event, shouldCreateSubscription bool,
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

			subs, innerErr := subRepo.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)
			if innerErr != nil {
				return subscriptions, &EndpointError{Err: fmt.Errorf("error fetching subscriptions for event type"), delay: defaultDelay}
			}

			if len(subs) == 0 && shouldCreateSubscription {
				genSubs := generateSubscription(project, endpoint)
				createSubErr := subRepo.CreateSubscription(ctx, project.UID, genSubs)
				if createSubErr != nil {
					return subscriptions, &EndpointError{Err: fmt.Errorf("error creating subscription for endpoint: %v", createSubErr), delay: defaultDelay}
				}

				subscriptions = append(subscriptions, *genSubs)
				return subscriptions, nil
			}

			matchedSubs, innerErr := matchSubscriptions(ctx, string(event.EventType), subs, filterRepo)
			if innerErr != nil {
				return subscriptions, &EndpointError{Err: fmt.Errorf("error matching subscriptions for event type: %v", innerErr), delay: defaultDelay}
			}

			matchedSubs, innerErr = matchSubscriptionsUsingFilter(ctx, event, subRepo, filterRepo, licenser, matchedSubs, false)
			if innerErr != nil {
				return subscriptions, &EndpointError{Err: fmt.Errorf("error fetching subscriptions for event type: %v", innerErr), delay: defaultDelay}
			}

			subscriptions = append(subscriptions, matchedSubs...)
		}
	} else if project.Type == datastore.IncomingProject {
		subscriptions, err = subRepo.FindSubscriptionsBySourceID(ctx, project.UID, event.SourceID)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}

		if len(subscriptions) > 0 {
			matchedSubs, innerErr := matchSubscriptions(ctx, string(event.EventType), subscriptions, filterRepo)
			if innerErr != nil {
				return subscriptions, &EndpointError{Err: fmt.Errorf("error matching subscriptions for event type: %v", innerErr), delay: defaultDelay}
			}

			matchedSubs, innerErr = matchSubscriptionsUsingFilter(ctx, event, subRepo, filterRepo, licenser, matchedSubs, false)
			if innerErr != nil {
				return subscriptions, &EndpointError{Err: fmt.Errorf("error fetching subscriptions for event type: %v", innerErr), delay: defaultDelay}
			}

			subscriptions = matchedSubs
		}

		subscriptions, err = matchSubscriptionsUsingFilter(ctx, event, subRepo, filterRepo, licenser, subscriptions, false)
		if err != nil {
			log.WithError(err).Error("error find a matching subscription for this source")
			return subscriptions, &EndpointError{Err: fmt.Errorf("error find a matching subscription for this source: %v", err), delay: defaultDelay}
		}
	}

	return subscriptions, nil
}

func matchSubscriptionsUsingFilter(ctx context.Context, e *datastore.Event, subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository, licenser license.Licenser, subscriptions []datastore.Subscription, soft bool) ([]datastore.Subscription, error) {
	if !licenser.AdvancedSubscriptions() {
		return subscriptions, nil
	}

	// fmt.Printf("matched %+v\n", subscriptions)

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
	// ]
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

		// First check if there's a specific filter for this event type
		filter, innerErr := filterRepo.FindFilterBySubscriptionAndEventType(ctx, s.UID, string(e.EventType))
		if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() && soft {
			log.WithError(innerErr).Errorf("failed to find filter for subscription (%s) and event type (%s)", s.UID, e.EventType)
			continue
		} else if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() {
			return nil, innerErr
		}

		// If no specific filter found, try to find a catch-all filter
		if filter == nil {
			filter, innerErr = filterRepo.FindFilterBySubscriptionAndEventType(ctx, s.UID, "*")
			if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() && soft {
				log.WithError(innerErr).Errorf("failed to find catch-all filter for subscription (%s)", s.UID)
				continue
			} else if innerErr != nil && !errors.Is(innerErr, datastore.ErrFilterNotFound) {
				return nil, innerErr
			}
		}

		// If no filter found at all, or filter has no conditions, match the subscription
		if filter == nil || (len(filter.Body) == 0 && len(filter.Headers) == 0) {
			matched = append(matched, *s)
			fmt.Printf("empty match: %+v %s\n", filter.EventType, s.UID)
			continue
		}

		isBodyMatched, innerErr := subRepo.CompareFlattenedPayload(ctx, flatPayload, filter.Body, true)
		if innerErr != nil && soft {
			log.WithError(innerErr).Errorf("subcription (%s) failed to match body", s.UID)
			continue
		} else if innerErr != nil {
			return nil, innerErr
		}

		isHeaderMatched, innerErr := subRepo.CompareFlattenedPayload(ctx, headers, filter.Headers, true)
		if innerErr != nil && soft {
			log.WithError(innerErr).Errorf("subscription (%s) failed to match header", s.UID)
			continue
		} else if innerErr != nil {
			return nil, innerErr
		}

		isMatched := isHeaderMatched && isBodyMatched

		if isMatched {
			fmt.Printf("m_bool: %v ?? b_bool: %v, h_bool: %v ?? h: %v hh: %v\n", isMatched, isBodyMatched, isHeaderMatched, headers, filter.Headers)
			matched = append(matched, *s)
		}

	}

	return matched, nil
}

func matchSubscriptions(ctx context.Context, eventType string, subscriptions []datastore.Subscription, filterRepo datastore.FilterRepository) ([]datastore.Subscription, error) {
	var matched []datastore.Subscription
	for _, sub := range subscriptions {
		// Check if there's a specific filter for this event type
		filter, err := filterRepo.FindFilterBySubscriptionAndEventType(ctx, sub.UID, eventType)
		if err != nil && err.Error() != datastore.ErrFilterNotFound.Error() {
			return nil, err
		}

		// If a specific filter exists, add the subscription
		if filter != nil {
			matched = append(matched, sub)
			continue
		}

		// Check for a catch-all filter
		filter, err = filterRepo.FindFilterBySubscriptionAndEventType(ctx, sub.UID, "*")
		if err != nil && err.Error() != datastore.ErrFilterNotFound.Error() {
			return nil, err
		}

		// If a catch-all filter exists, add the subscription
		if filter != nil {
			matched = append(matched, sub)
		}
	}

	return matched, nil
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
	endpoints := make([]datastore.Endpoint, 0)

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
