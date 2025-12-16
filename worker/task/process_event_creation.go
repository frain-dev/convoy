package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/transform"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

// OAuth2TokenService is an interface for getting OAuth2 authorization headers.
type OAuth2TokenService interface {
	GetAuthorizationHeader(context.Context, *datastore.Endpoint) (string, error)
}

// getOAuth2TokenService performs type assertion on oauth2TokenService.

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

type EventProcessorDeps struct {
	EndpointRepo               datastore.EndpointRepository
	EventRepo                  datastore.EventRepository
	ProjectRepo                datastore.ProjectRepository
	EventQueue                 queue.Queuer
	SubRepo                    datastore.SubscriptionRepository
	FilterRepo                 datastore.FilterRepository
	Licenser                   license.Licenser
	TracerBackend              tracer.Backend
	OAuth2TokenService         OAuth2TokenService
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
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
		err = json.Unmarshal(t.Payload(), &createEvent)
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
		err = updateEventMetadata(channel, event, createEvent.CreateSubscription)
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

func ProcessEventCreation(deps EventProcessorDeps) func(context.Context, *asynq.Task) error {
	ch := &DefaultEventChannel{}

	return ProcessEventCreationByChannel(
		ch,
		deps.EndpointRepo,
		deps.EventRepo,
		deps.ProjectRepo,
		deps.EventQueue,
		deps.SubRepo,
		deps.FilterRepo,
		deps.Licenser,
		deps.TracerBackend,
		deps.OAuth2TokenService,
	)
}

type WriteEventDeliveriesToQueueOptions struct {
	Subscriptions              []datastore.Subscription
	Event                      *datastore.Event
	Project                    *datastore.Project
	EventDeliveryRepo          datastore.EventDeliveryRepository
	EventQueue                 queue.Queuer
	EndpointRepo               datastore.EndpointRepository
	Licenser                   license.Licenser
	OAuth2TokenService         OAuth2TokenService
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
}

func writeEventDeliveriesToQueue(ctx context.Context, opts WriteEventDeliveriesToQueueOptions) error {
	ec := &EventDeliveryConfig{project: opts.Project}

	eventDeliveries := make([]*datastore.EventDelivery, 0)
	for _, s := range opts.Subscriptions {
		ec.subscription = &s
		headers := opts.Event.Headers

		if s.Type == datastore.SubscriptionTypeAPI {
			endpoint, err := opts.EndpointRepo.FindEndpointByID(ctx, s.EndpointID, opts.Project.UID)
			if err != nil {
				if errors.Is(err, datastore.ErrEndpointNotFound) {
					continue
				}

				return &EndpointError{Err: fmt.Errorf("CODE: 1006, err: %s", err.Error()), delay: defaultDelay}
			}

			authType := ""
			hasOAuth2 := false
			if endpoint.Authentication != nil {
				authType = string(endpoint.Authentication.Type)
				hasOAuth2 = endpoint.Authentication.OAuth2 != nil
			}
			log.FromContext(ctx).WithFields(log.Fields{
				"endpoint.id":        endpoint.UID,
				"has_authentication": endpoint.Authentication != nil,
				"auth_type":          authType,
				"has_oauth2":         hasOAuth2,
			}).Debug("Processing endpoint authentication")

			if endpoint.Authentication != nil {
				switch endpoint.Authentication.Type {
				case datastore.APIKeyAuthentication:
					headers = make(httpheader.HTTPHeader)
					headers[endpoint.Authentication.ApiKey.HeaderName] = []string{endpoint.Authentication.ApiKey.HeaderValue}
					headers.MergeHeaders(opts.Event.Headers)
				case datastore.OAuth2Authentication:
					// Check feature flag for OAuth2 using project's organisation ID
					oauth2Enabled := opts.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, opts.FeatureFlagFetcher, opts.EarlyAdopterFeatureFetcher, opts.Project.OrganisationID)
					if !oauth2Enabled {
						log.FromContext(ctx).Warn("Endpoint has OAuth2 configured but feature flag is disabled, skipping OAuth2 authentication")
						// Continue without OAuth2 authentication if feature flag is disabled
					} else if opts.OAuth2TokenService == nil {
						log.FromContext(ctx).Error("OAuth2 token service is nil")
					} else {
						authHeader, err := opts.OAuth2TokenService.GetAuthorizationHeader(ctx, endpoint)
						if err != nil {
							log.FromContext(ctx).WithError(err).Error("failed to get OAuth2 authorization header")
						} else {
							headers = make(httpheader.HTTPHeader)
							headers["Authorization"] = []string{authHeader}
							headers.MergeHeaders(opts.Event.Headers)
							log.FromContext(ctx).WithFields(log.Fields{
								"endpoint.id": endpoint.UID,
							}).Info("OAuth2 authorization header retrieved and added to headers")
						}
					}
				default:
					log.FromContext(ctx).WithFields(log.Fields{
						"endpoint.id": endpoint.UID,
						"auth_type":   endpoint.Authentication.Type,
					}).Debug("Unknown authentication type, skipping")
				}
			} else {
				log.FromContext(ctx).WithFields(log.Fields{
					"endpoint.id": endpoint.UID,
				}).Debug("Endpoint has no authentication configured")
			}

			s.Endpoint = endpoint
		}

		rc, err := ec.RetryConfig()
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		raw := opts.Event.Raw
		data := opts.Event.Data

		if s.Function.Ptr() != nil && !util.IsStringEmpty(s.Function.String) && opts.Licenser.Transformations() {
			var payload map[string]interface{}
			err = json.Unmarshal(opts.Event.Data, &payload)
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
			EventType:      opts.Event.EventType,
			Metadata:       metadata,
			ProjectID:      opts.Project.UID,
			EventID:        opts.Event.UID,
			EndpointID:     s.EndpointID,
			DeviceID:       s.DeviceID,
			Headers:        headers,
			IdempotencyKey: opts.Event.IdempotencyKey,
			URLQueryParams: opts.Event.URLQueryParams,
			Status:         getEventDeliveryStatus(ctx, &s, s.Endpoint),
			AcknowledgedAt: null.TimeFrom(time.Now()),
			DeliveryMode:   s.DeliveryMode,
		}

		if s.Type == datastore.SubscriptionTypeCLI {
			opts.Event.Endpoints = []string{}
			eventDelivery.CLIMetadata = &datastore.CLIMetadata{
				EventType: string(opts.Event.EventType),
				SourceID:  opts.Event.SourceID,
			}
		}

		eventDeliveries = append(eventDeliveries, eventDelivery)
	}

	err := opts.EventDeliveryRepo.CreateEventDeliveries(ctx, eventDeliveries)
	if err != nil {
		return &EndpointError{Err: fmt.Errorf("CODE: 1008, err: %s", err.Error()), delay: defaultDelay}
	}

	for i, eventDelivery := range eventDeliveries {
		s := opts.Subscriptions[i]
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
				err = opts.EventQueue.Write(convoy.EventProcessor, convoy.EventQueue, job)
				if err != nil {
					log.FromContext(ctx).WithError(err).Errorf("[asynq]: an error occurred sending event delivery to be dispatched")
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

	switch project.Type {
	case datastore.OutgoingProject:
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
	case datastore.IncomingProject:
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

	for i := range subscriptions {
		sub := &subscriptions[i]

		log.FromContext(ctx).WithFields(log.Fields{
			"event.id":        e.UID,
			"subscription.id": sub.UID,
		}).Debug("matching subscription")

		// First check if there's a specific filter for this event type
		filter, innerErr := filterRepo.FindFilterBySubscriptionAndEventType(ctx, sub.UID, string(e.EventType))
		if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() && soft {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
			}).WithError(innerErr).Error("failed to find filter subscription")
			continue
		} else if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
			}).WithError(innerErr).Error("fiter not found")
			return nil, innerErr
		}

		// If no specific filter found, try to find a catch-all filter
		if filter == nil {
			filter, innerErr = filterRepo.FindFilterBySubscriptionAndEventType(ctx, sub.UID, "*")
			if innerErr != nil && innerErr.Error() != datastore.ErrFilterNotFound.Error() && soft {
				log.FromContext(ctx).WithFields(log.Fields{
					"event.id":        e.UID,
					"subscription.id": sub.UID,
				}).WithError(innerErr).Error("failed to find catch-all filter")
				continue
			} else if innerErr != nil && !errors.Is(innerErr, datastore.ErrFilterNotFound) {
				log.FromContext(ctx).WithFields(log.Fields{
					"event.id":        e.UID,
					"subscription.id": sub.UID,
				}).WithError(innerErr).Error("catch-all filter not found")
				return nil, innerErr
			}
		}

		// If no filter found at all, or filter has no conditions, match the subscription
		if filter == nil || (len(filter.Body) == 0 && len(filter.Headers) == 0) {
			matched = append(matched, *sub)
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
			}).Debug("subscription event type matched passed")
			continue
		}

		isBodyMatched, innerErr := subRepo.CompareFlattenedPayload(ctx, flatPayload, filter.Body, true)
		if innerErr != nil && soft {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
				"soft":            soft,
			}).WithError(innerErr).Error("subscription failed to match body")
			continue
		} else if innerErr != nil {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
				"soft":            soft,
			}).WithError(innerErr).Error("subscription failed to match body")
			return nil, innerErr
		}

		isHeaderMatched, innerErr := subRepo.CompareFlattenedPayload(ctx, headers, filter.Headers, true)
		if innerErr != nil && soft {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
				"soft":            soft,
			}).WithError(innerErr).Error("subscription failed to match header")
			continue
		} else if innerErr != nil {
			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
				"soft":            soft,
			}).WithError(innerErr).Error("subscription failed to match header")
			return nil, innerErr
		}

		isMatched := isHeaderMatched && isBodyMatched

		if isMatched {
			matched = append(matched, *sub)

			log.FromContext(ctx).WithFields(log.Fields{
				"event.id":        e.UID,
				"subscription.id": sub.UID,
			}).Debug("subscription filter matched passed")
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
) datastore.EventDeliveryStatus {
	switch subscription.Type {
	case datastore.SubscriptionTypeAPI:
		if endpoint.Status != datastore.ActiveEndpointStatus {
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

	endpointIDs := make([]string, 0, len(endpoints))
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
