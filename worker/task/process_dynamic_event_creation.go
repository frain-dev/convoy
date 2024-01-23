package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

func ProcessDynamicEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var dynamicEvent models.DynamicEvent

		err := msgpack.DecodeMsgPack(t.Payload(), &dynamicEvent)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &dynamicEvent)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		project, err := projectRepo.FetchProjectByID(ctx, dynamicEvent.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		endpoint, err := findEndpoint(ctx, project, endpointRepo, &dynamicEvent)
		if err != nil {
			return err
		}

		s, err := findDynamicSubscription(ctx, &dynamicEvent, subRepo, project, endpoint)
		if err != nil {
			return err
		}

		var isDuplicate bool
		if len(dynamicEvent.IdempotencyKey) > 0 {
			events, err := eventRepo.FindEventsByIdempotencyKey(ctx, dynamicEvent.ProjectID, dynamicEvent.IdempotencyKey)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			isDuplicate = len(events) > 0
		}

		event := &datastore.Event{
			UID:              ulid.Make().String(),
			EventType:        datastore.EventType(dynamicEvent.EventType),
			ProjectID:        project.UID,
			Endpoints:        []string{endpoint.UID},
			Data:             dynamicEvent.Data,
			IdempotencyKey:   dynamicEvent.IdempotencyKey,
			IsDuplicateEvent: isDuplicate,
			Raw:              string(dynamicEvent.Data),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		if event.IsDuplicateEvent {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		ec := &EventDeliveryConfig{project: project}

		ec.subscription = s
		headers := event.Headers

		if s.Type == datastore.SubscriptionTypeAPI {
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

		raw := event.Raw
		data := event.Data

		if s.Function.Ptr() != nil && !util.IsStringEmpty(s.Function.String) {
			var payload map[string]interface{}
			err = json.Unmarshal(event.Data, &payload)
			if err != nil {
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			mutated, _, err := subRepo.TransformPayload(ctx, s.Function.String, payload)
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
			Metadata:       metadata,
			ProjectID:      project.UID,
			EventID:        event.UID,
			EndpointID:     s.EndpointID,
			DeviceID:       s.DeviceID,
			Headers:        headers,
			IdempotencyKey: event.IdempotencyKey,

			Status:           getEventDeliveryStatus(ctx, s, s.Endpoint, deviceRepo),
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
				ProjectID:       eventDelivery.ProjectID,
			}

			data, err := msgpack.EncodeMsgPack(payload)
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
					log.WithError(err).Errorf("[asynq]: an error occurred sending event delivery to be dispatched")
				}
			}

			if s.Type == datastore.SubscriptionTypeCLI {
				err = eventQueue.Write(convoy.StreamCliEventsProcessor, convoy.StreamQueue, job)
				if err != nil {
					log.WithError(err).Error("[asynq]: an error occurred sending event delivery to the stream queue")
				}
			}
		}

		return nil
	}
}

func findEndpoint(ctx context.Context, project *datastore.Project, endpointRepo datastore.EndpointRepository, dynamicEvent *models.DynamicEvent) (*datastore.Endpoint, error) {
	endpoint, err := endpointRepo.FindEndpointByTargetURL(ctx, project.UID, dynamicEvent.URL)

	switch {
	case errors.Is(err, datastore.ErrEndpointNotFound):
		endpoint = &datastore.Endpoint{
			UID:                ulid.Make().String(),
			ProjectID:          project.UID,
			Title:              fmt.Sprintf("endpoint-%s", uuid.NewString()),
			TargetURL:          dynamicEvent.URL,
			RateLimit:          convoy.RATE_LIMIT,
			HttpTimeout:        convoy.HTTP_TIMEOUT,
			AdvancedSignatures: true,
			RateLimitDuration:  convoy.RATE_LIMIT_DURATION,
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
			subscription.FilterConfig.EventTypes = dynamicEvent.EventTypes
		}
		err = subRepo.UpdateSubscription(ctx, project.UID, subscription)
		if err != nil {
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
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
