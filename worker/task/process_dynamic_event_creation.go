package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/guregu/null.v4"
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
			Headers:          getCustomHeaders(dynamicEvent.CustomHeaders),
			IsDuplicateEvent: isDuplicate,
			Raw:              string(dynamicEvent.Data),
			AcknowledgedAt:   null.TimeFrom(time.Now()),
		}

		err = eventRepo.CreateEvent(ctx, event)
		if err != nil {
			return &EndpointError{Err: err, delay: 10 * time.Second}
		}

		if event.IsDuplicateEvent {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		return writeEventDeliveriesToQueue(
			ctx, []datastore.Subscription{*s}, event, project, eventDeliveryRepo,
			eventQueue, deviceRepo, endpointRepo,
		)
	}
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
