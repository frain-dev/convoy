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
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/msgpack"
	endpointurl "github.com/frain-dev/convoy/pkg/url"
	"github.com/frain-dev/convoy/util"
)

type DynamicEventChannel struct {
}

var (
	errDynamicURLTemplateNotConcrete   = errors.New("dynamic event URL must be concrete, not an endpoint URL template")
	errDynamicURLTemplateNoMatch       = errors.New("dynamic URL does not match any configured endpoint URL template")
	errDynamicURLTemplateMultipleMatch = errors.New("multiple endpoint URL templates match dynamic URL")
	errDynamicURLTemplateFeatureLookup = errors.New("endpoint URL template feature lookup failed")
)

func NewDynamicEventChannel() *DynamicEventChannel {
	return &DynamicEventChannel{}
}

func (d *DynamicEventChannel) GetConfig() *EventChannelConfig {
	return &EventChannelConfig{
		Channel:      "dynamic",
		DefaultDelay: defaultDelay,
	}
}

func (d *DynamicEventChannel) CreateEvent(ctx context.Context, t *asynq.Task, channel EventChannel, args EventChannelArgs) (*datastore.Event, error) {
	// Start a new trace span for event creation
	attributes := map[string]interface{}{
		"event.type": "dynamic.event.creation",
		"channel":    channel,
	}

	var dynamicEvent models.DynamicEvent
	err := msgpack.DecodeMsgPack(t.Payload(), &dynamicEvent)
	if err != nil {
		err := json.Unmarshal(t.Payload(), &dynamicEvent)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	attributes["project.id"] = dynamicEvent.ProjectID
	attributes["event.id"] = dynamicEvent.EventID

	if util.IsStringEmpty(dynamicEvent.EventID) {
		dynamicEvent.EventID = ulid.Make().String() // legacy events
	}

	project, err := args.projectRepo.FetchProjectByID(ctx, dynamicEvent.ProjectID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}
	if err = license.EnsureProjectEnabled(args.licenser, project.UID); err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
		return nil, &EndpointError{Err: err, delay: defaultEventDelay}
	}

	var isDuplicate bool
	if len(dynamicEvent.IdempotencyKey) > 0 {
		isDuplicate, err = args.eventRepo.FindEventsByIdempotencyKey(ctx, dynamicEvent.ProjectID, dynamicEvent.IdempotencyKey)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}
	}

	metadata := make(map[string]string)
	metadata["channel"] = channel.GetConfig().Channel
	metadata["delay"] = strconv.FormatInt(int64(channel.GetConfig().DefaultDelay), 10)
	payload, _ := json.Marshal(dynamicEvent)
	metadata["dynamicPayload"] = string(payload)
	m, err := json.Marshal(metadata)
	if err != nil {
		args.logger.Error("failed to marshal metadata for event", "error", err)
		tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	event := &datastore.Event{
		UID:              dynamicEvent.EventID,
		EventType:        datastore.EventType(dynamicEvent.EventType),
		ProjectID:        project.UID,
		Data:             dynamicEvent.Data,
		IdempotencyKey:   dynamicEvent.IdempotencyKey,
		Headers:          getCustomHeaders(dynamicEvent.CustomHeaders),
		IsDuplicateEvent: isDuplicate,
		Metadata:         string(m),
		Raw:              "", // Skip Raw duplication - Data field is canonical (reduces payload size)
		AcknowledgedAt:   null.TimeFrom(time.Now()),
	}

	err = args.eventRepo.CreateEvent(ctx, event)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventCreationError, attributes)
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	tracer.AddEvent(ctx, tracer.EventDynamicEventCreationSuccess, attributes)
	return event, nil
}

func (d *DynamicEventChannel) MatchSubscriptions(ctx context.Context, metadata EventChannelMetadata, args EventChannelArgs) (*EventChannelSubResponse, error) {
	// Start a new trace span for subscription matching
	attributes := map[string]interface{}{
		"event.type": "dynamic.event.subscription.matching",
		"event.id":   metadata.Event.UID,
		"channel":    metadata.Config.Channel,
	}

	response := EventChannelSubResponse{}

	project, err := args.projectRepo.FetchProjectByID(ctx, metadata.Event.ProjectID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}
	if err = license.EnsureProjectEnabled(args.licenser, project.UID); err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, &EndpointError{Err: err, delay: defaultEventDelay}
	}

	event, err := args.eventRepo.FindEventByID(ctx, project.UID, metadata.Event.UID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	err = args.eventRepo.UpdateEventStatus(ctx, event, datastore.ProcessingStatus)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, err
	}

	var dynamicEvent models.DynamicEvent
	if !util.IsStringEmpty(event.Metadata) {
		var m map[string]string
		err := json.Unmarshal([]byte(event.Metadata), &m)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
		p := m["dynamicPayload"]
		err = json.Unmarshal([]byte(p), &dynamicEvent)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	endpoint, err := findEndpoint(ctx, project, args, &dynamicEvent)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		if isDynamicURLTemplateValidationError(err) {
			if updateErr := args.eventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus); updateErr != nil {
				return nil, updateErr
			}
			return &EventChannelSubResponse{Event: event, Project: project, IsDuplicateEvent: true}, nil
		}
		return nil, err
	}

	s, err := findDynamicSubscription(ctx, &dynamicEvent, args.subRepo, project, endpoint)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, err
	}

	err = args.eventRepo.UpdateEventEndpoints(ctx, event, []string{endpoint.UID})
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingError, attributes)
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	response.Event = event
	response.Project = project
	response.Subscriptions = []datastore.Subscription{*s}
	response.IsDuplicateEvent = event.IsDuplicateEvent
	if endpoint.Url != dynamicEvent.URL {
		response.TargetURL = dynamicEvent.URL
	}

	tracer.AddEvent(ctx, tracer.EventDynamicEventSubscriptionMatchingOK, attributes)
	return &response, nil
}

func ProcessDynamicEventCreation(deps EventProcessorDeps) func(context.Context, *asynq.Task) error {
	ch := &DynamicEventChannel{}

	return ProcessEventCreationByChannel(
		ch,
		deps.EndpointRepo,
		deps.EventRepo,
		deps.ProjectRepo,
		deps.EventQueue,
		deps.SubRepo,
		deps.FilterRepo,
		deps.Licenser,
		deps.OAuth2TokenService,
		deps.FeatureFlag,
		deps.FeatureFlagFetcher,
		deps.EarlyAdopterFeatureFetcher,
		deps.Logger,
	)
}

func findEndpoint(ctx context.Context, project *datastore.Project, args EventChannelArgs, dynamicEvent *models.DynamicEvent) (*datastore.Endpoint, error) {
	if endpointurl.ContainsTemplate(dynamicEvent.URL) {
		return nil, &EndpointError{Err: errDynamicURLTemplateNotConcrete, delay: 10 * time.Second}
	}

	endpoint, err := args.endpointRepo.FindEndpointByTargetURL(ctx, project.UID, dynamicEvent.URL)
	if err == nil {
		return endpoint, nil
	}

	switch {
	case errors.Is(err, datastore.ErrEndpointNotFound):
		endpointURLTemplatesEnabled, featureErr := endpointURLTemplatesEnabled(ctx, args, project)
		if featureErr != nil {
			foundTemplates, templateErr := hasValidEndpointURLTemplates(ctx, args, project.UID)
			if templateErr != nil {
				return nil, &EndpointError{Err: templateErr, delay: 10 * time.Second}
			}
			if foundTemplates {
				return nil, &EndpointError{Err: fmt.Errorf("%w: %w", errDynamicURLTemplateFeatureLookup, featureErr), delay: 10 * time.Second}
			}
		} else if endpointURLTemplatesEnabled {
			endpoint, foundTemplates, err := findTemplatedEndpoint(ctx, args, project.UID, dynamicEvent.URL)
			if err != nil {
				return nil, err
			}
			if endpoint != nil {
				return endpoint, nil
			}
			if foundTemplates {
				return nil, &EndpointError{Err: errDynamicURLTemplateNoMatch, delay: 10 * time.Second}
			}
		}

		uid := ulid.Make().String()
		endpoint = &datastore.Endpoint{
			UID:                uid,
			ProjectID:          project.UID,
			Name:               fmt.Sprintf("endpoint-%s", uid),
			Url:                dynamicEvent.URL,
			HttpTimeout:        convoy.HTTP_TIMEOUT,
			AdvancedSignatures: true,
			Status:             datastore.ActiveEndpointStatus,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		sc := dynamicEvent.Secret
		if util.IsStringEmpty(sc) {
			sc, err = util.GenerateSecret()
			if err != nil {
				return nil, &EndpointError{Err: err, delay: 10 * time.Second}
			}
		}

		endpoint.Secrets = []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     sc,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		err = args.endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
		if err != nil {
			args.logger.Error("failed to create endpoint", "error", err)
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}

		return endpoint, nil
	default:
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}
}

func endpointURLTemplatesEnabled(ctx context.Context, args EventChannelArgs, project *datastore.Project) (bool, error) {
	if args.featureFlag == nil || !args.licenser.EndpointURLTemplates() {
		return false, nil
	}

	if args.earlyAdopterFeatureFetcher == nil {
		return args.featureFlag.CanAccessFeature(fflag.EndpointURLTemplates), nil
	}

	feature, err := args.earlyAdopterFeatureFetcher.FetchEarlyAdopterFeature(ctx, project.OrganisationID, string(fflag.EndpointURLTemplates))
	if err != nil {
		return false, err
	}
	return feature.Enabled, nil
}

func isDynamicURLTemplateValidationError(err error) bool {
	endpointErr, ok := err.(*EndpointError)
	if !ok {
		return false
	}

	return errors.Is(endpointErr.Err, errDynamicURLTemplateNotConcrete) ||
		errors.Is(endpointErr.Err, errDynamicURLTemplateNoMatch) ||
		errors.Is(endpointErr.Err, errDynamicURLTemplateMultipleMatch) ||
		errors.Is(endpointErr.Err, errDynamicURLTemplateFeatureLookup)
}

func hasValidEndpointURLTemplates(ctx context.Context, args EventChannelArgs, projectID string) (bool, error) {
	candidates, err := args.endpointRepo.FindEndpointsWithURLTemplates(ctx, projectID)
	if err != nil {
		return false, err
	}

	for i := range candidates {
		if endpointurl.ContainsTemplate(candidates[i].Url) {
			return true, nil
		}
	}

	return false, nil
}

func findTemplatedEndpoint(ctx context.Context, args EventChannelArgs, projectID, concreteURL string) (*datastore.Endpoint, bool, error) {
	candidates, err := args.endpointRepo.FindEndpointsWithURLTemplates(ctx, projectID)
	if err != nil {
		return nil, false, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	var matchedEndpoint *datastore.Endpoint
	foundValidTemplate := false
	for i := range candidates {
		candidate := &candidates[i]
		if !endpointurl.ContainsTemplate(candidate.Url) {
			continue
		}
		foundValidTemplate = true
		match, err := endpointurl.TemplateMatches(candidate.Url, concreteURL)
		if err != nil {
			args.logger.WarnContext(ctx, "skipping invalid endpoint URL template", "endpoint_id", candidate.UID, "error", err)
			continue
		}
		if match {
			if matchedEndpoint != nil {
				return nil, true, &EndpointError{Err: errDynamicURLTemplateMultipleMatch, delay: 10 * time.Second}
			}
			matchedEndpoint = candidate
		}
	}

	return matchedEndpoint, foundValidTemplate, nil
}

func findDynamicSubscription(ctx context.Context, dynamicEvent *models.DynamicEvent,
	subRepo datastore.SubscriptionRepository, project *datastore.Project, endpoint *datastore.Endpoint) (*datastore.Subscription, error) {
	subscriptions, err := subRepo.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)

	var subscription *datastore.Subscription

	if len(subscriptions) == 0 && err == nil {
		err = datastore.ErrSubscriptionNotFound
	}

	switch {
	case err == nil:
		subscription = &subscriptions[0]
		err = syncDynamicSubscriptionEventTypes(ctx, dynamicEvent, subRepo, project, subscription)
		if err != nil {
			return nil, err
		}
	case errors.Is(err, datastore.ErrSubscriptionNotFound):
		subscription = &datastore.Subscription{
			UID:        ulid.Make().String(),
			ProjectID:  project.UID,
			Name:       fmt.Sprintf("dynamic-subscription-%s", endpoint.UID),
			Type:       datastore.SubscriptionTypeAPI,
			EndpointID: endpoint.UID,
			FilterConfig: &datastore.FilterConfiguration{
				EventTypes: dynamicEvent.EventTypes,
			},
			RetryConfig:     &datastore.DefaultRetryConfig,
			AlertConfig:     &datastore.DefaultAlertConfig,
			RateLimitConfig: &datastore.DefaultRateLimitConfig,

			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		subscription, err = subRepo.FindOrCreateDynamicSubscription(ctx, project.UID, subscription)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}
		err = syncDynamicSubscriptionEventTypes(ctx, dynamicEvent, subRepo, project, subscription)
		if err != nil {
			return nil, err
		}
	default:
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	return subscription, nil
}

func syncDynamicSubscriptionEventTypes(ctx context.Context, dynamicEvent *models.DynamicEvent,
	subRepo datastore.SubscriptionRepository, project *datastore.Project, subscription *datastore.Subscription) error {
	if len(dynamicEvent.EventTypes) == 0 {
		return nil
	}

	if subscription.FilterConfig == nil {
		subscription.FilterConfig = &datastore.FilterConfiguration{}
	}
	subscription.FilterConfig.EventTypes = dynamicEvent.EventTypes

	err := subRepo.UpdateSubscription(ctx, project.UID, subscription)
	if err != nil {
		return &EndpointError{Err: err, delay: 10 * time.Second}
	}

	return nil
}

func getCustomHeaders(customHeaders map[string]string) httpheader.HTTPHeader {
	var headers map[string][]string

	if customHeaders != nil {
		headers = make(map[string][]string)

		for key, value := range customHeaders {
			headers[key] = []string{value}
		}
	}

	return headers
}
