package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

type DynamicEventChannel struct {
}

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
	startTime := time.Now()
	attributes := map[string]interface{}{
		"event.type": "dynamic.event.creation",
		"channel":    channel,
	}

	var dynamicEvent models.DynamicEvent
	err := msgpack.DecodeMsgPack(t.Payload(), &dynamicEvent)
	if err != nil {
		err := json.Unmarshal(t.Payload(), &dynamicEvent)
		if err != nil {
			args.tracerBackend.Capture(ctx, "dynamic.event.creation.error", attributes, startTime, time.Now())
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
		args.tracerBackend.Capture(ctx, "dynamic.event.creation.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	var isDuplicate bool
	if len(dynamicEvent.IdempotencyKey) > 0 {
		events, err := args.eventRepo.FindEventsByIdempotencyKey(ctx, dynamicEvent.ProjectID, dynamicEvent.IdempotencyKey)
		if err != nil {
			args.tracerBackend.Capture(ctx, "dynamic.event.creation.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}

		isDuplicate = len(events) > 0
	}

	metadata := make(map[string]string)
	metadata["channel"] = channel.GetConfig().Channel
	metadata["delay"] = strconv.FormatInt(int64(channel.GetConfig().DefaultDelay), 10)
	payload, _ := json.Marshal(dynamicEvent)
	metadata["dynamicPayload"] = string(payload)
	m, err := json.Marshal(metadata)
	if err != nil {
		log.WithError(err).Error("failed to marshal metadata for event")
		args.tracerBackend.Capture(ctx, "dynamic.event.creation.error", attributes, startTime, time.Now())
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
		Raw:              string(dynamicEvent.Data),
		AcknowledgedAt:   null.TimeFrom(time.Now()),
	}

	err = args.eventRepo.CreateEvent(ctx, event)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.creation.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	args.tracerBackend.Capture(ctx, "dynamic.event.creation.success", attributes, startTime, time.Now())
	return event, nil
}

func (d *DynamicEventChannel) MatchSubscriptions(ctx context.Context, metadata EventChannelMetadata, args EventChannelArgs) (*EventChannelSubResponse, error) {
	// Start a new trace span for subscription matching
	startTime := time.Now()
	attributes := map[string]interface{}{
		"event.type": "dynamic.event.subscription.matching",
		"event.id":   metadata.Event.UID,
		"channel":    metadata.Config.Channel,
	}

	response := EventChannelSubResponse{}

	project, err := args.projectRepo.FetchProjectByID(ctx, metadata.Event.ProjectID)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	event, err := args.eventRepo.FindEventByID(ctx, project.UID, metadata.Event.UID)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	err = args.eventRepo.UpdateEventStatus(ctx, event, datastore.ProcessingStatus)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, err
	}

	var dynamicEvent models.DynamicEvent
	if !util.IsStringEmpty(event.Metadata) {
		var m map[string]string
		err := json.Unmarshal([]byte(event.Metadata), &m)
		if err != nil {
			args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
		p := m["dynamicPayload"]
		err = json.Unmarshal([]byte(p), &dynamicEvent)
		if err != nil {
			args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
			return nil, &EndpointError{Err: err, delay: defaultDelay}
		}
	}

	endpoint, err := findEndpoint(ctx, project, args.endpointRepo, &dynamicEvent)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, err
	}

	s, err := findDynamicSubscription(ctx, &dynamicEvent, args.subRepo, project, endpoint)
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, err
	}

	err = args.eventRepo.UpdateEventEndpoints(ctx, event, []string{endpoint.UID})
	if err != nil {
		args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.error", attributes, startTime, time.Now())
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	response.Event = event
	response.Project = project
	response.Subscriptions = []datastore.Subscription{*s}
	response.IsDuplicateEvent = event.IsDuplicateEvent

	args.tracerBackend.Capture(ctx, "dynamic.event.subscription.matching.success", attributes, startTime, time.Now())
	return &response, nil
}

func ProcessDynamicEventCreation(ch *DynamicEventChannel, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository, licenser license.Licenser, tracerBackend tracer.Backend) func(context.Context, *asynq.Task) error {
	return ProcessEventCreationByChannel(ch, endpointRepo, eventRepo, projectRepo, eventQueue, subRepo, licenser, tracerBackend)
}

func findEndpoint(ctx context.Context, project *datastore.Project, endpointRepo datastore.EndpointRepository, dynamicEvent *models.DynamicEvent) (*datastore.Endpoint, error) {
	endpoint, err := endpointRepo.FindEndpointByTargetURL(ctx, project.UID, dynamicEvent.URL)
	if err == nil {
		return endpoint, nil
	}

	switch {
	case errors.Is(err, datastore.ErrEndpointNotFound):
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

		err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
		if err != nil {
			log.WithError(err).Error("failed to create endpoint")
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}

		return endpoint, nil
	default:
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}
}

func findDynamicSubscription(ctx context.Context, dynamicEvent *models.DynamicEvent, subRepo datastore.SubscriptionRepository, project *datastore.Project, endpoint *datastore.Endpoint) (*datastore.Subscription, error) {
	subscriptions, err := subRepo.FindSubscriptionsByEndpointID(ctx, project.UID, endpoint.UID)

	var subscription *datastore.Subscription

	if len(subscriptions) == 0 && err == nil {
		err = datastore.ErrSubscriptionNotFound
	}

	switch {
	case err == nil:
		subscription = &subscriptions[0]
		if len(dynamicEvent.EventTypes) > 0 {
			if subscription.FilterConfig == nil {
				subscription.FilterConfig = &datastore.FilterConfiguration{}
			}
			subscription.FilterConfig.EventTypes = dynamicEvent.EventTypes

			err = subRepo.UpdateSubscription(ctx, project.UID, subscription)
			if err != nil {
				return nil, &EndpointError{Err: err, delay: 10 * time.Second}
			}
		}
	case errors.Is(err, datastore.ErrSubscriptionNotFound):
		subscription = &datastore.Subscription{
			UID:        ulid.Make().String(),
			ProjectID:  project.UID,
			Name:       fmt.Sprintf("subscription-%s", uuid.NewString()),
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

		err = subRepo.CreateSubscription(ctx, project.UID, subscription)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}
	default:
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	return subscription, nil
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
