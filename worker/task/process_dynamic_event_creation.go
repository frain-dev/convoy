package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
)

func ProcessDynamicEventCreation(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, search searcher.Searcher, deviceRepo datastore.DeviceRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var dynamicEvent models.DynamicEvent
		var event datastore.Event

		err := json.Unmarshal(t.Payload(), &dynamicEvent)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		var project *datastore.Project
		var subscriptions []datastore.Subscription

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

		endpoint, err := findEndpoint(ctx, project, endpointRepo, &dynamicEvent.Endpoint)
		if err != nil {
			return err
		}

		event.MatchedEndpoints = len(subscriptions)
		ec := &EventDeliveryConfig{project: project}

		for _, s := range subscriptions {
			ec.subscription = &s
			headers := event.Headers

			if s.Type == datastore.SubscriptionTypeAPI {
				endpoint, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, project.UID)
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
				log.WithError(err).Error("error occurred creating event delivery")
				return &EndpointError{Err: err, delay: 10 * time.Second}
			}

			if eventDelivery.Status != datastore.DiscardedEventStatus {
				payload := EventDelivery{
					EventDeliveryID: eventDelivery.UID,
					ProjectID:       project.UID,
				}

				data, err := json.Marshal(payload)
				if err != nil {
					log.WithError(err).Error("failed to marshal process event delivery payload")
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
				} else if s.Type == datastore.SubscriptionTypeCLI {
					err = eventQueue.Write(convoy.StreamCliEventsProcessor, convoy.StreamQueue, job)
					if err != nil {
						log.WithError(err).Error("[asynq]: an error occurred sending event delivery to the stream queue")
					}
				}
			}
		}

		eBytes, err := json.Marshal(event)
		if err != nil {
			log.Errorf("[asynq]: an error occurred marshalling event to be indexed %s", err)
		}

		job := &queue.Job{
			ID:      event.UID,
			Payload: eBytes,
			Delay:   5 * time.Second,
		}

		err = eventQueue.Write(convoy.IndexDocument, convoy.SearchIndexQueue, job)
		if err != nil {
			log.Errorf("[asynq]: an error occurred sending event to be indexed %s", err)
		}

		return nil
	}
}

func findEndpoint(ctx context.Context, project *datastore.Project, endpointRepo datastore.EndpointRepository, newEndpoint *models.Endpoint) (*datastore.Endpoint, error) {
	endpoint, err := endpointRepo.FindEndpointByTargetURL(ctx, project.UID, newEndpoint.URL)

	switch err {
	case nil:
		if !util.IsStringEmpty(newEndpoint.Description) {
			endpoint.Description = newEndpoint.Description
		}

		if !util.IsStringEmpty(newEndpoint.Description) {
			endpoint.Description = newEndpoint.Description
		}

		endpoint.Description = newEndpoint.Description

		endpoint.Title = newEndpoint.Name

		if !util.IsStringEmpty(newEndpoint.SupportEmail) {
			endpoint.SupportEmail = newEndpoint.SupportEmail
		}

		if !util.IsStringEmpty(newEndpoint.SlackWebhookURL) {
			endpoint.SlackWebhookURL = newEndpoint.SlackWebhookURL
		}

		if newEndpoint.RateLimit != 0 {
			endpoint.RateLimit = newEndpoint.RateLimit
		}

		if !util.IsStringEmpty(newEndpoint.RateLimitDuration) {
			duration, err := time.ParseDuration(newEndpoint.RateLimitDuration)
			if err != nil {
				return nil, err
			}

			endpoint.RateLimitDuration = duration.String()
		}

		if (newEndpoint.AdvancedSignatures != endpoint.AdvancedSignatures) && project.Type == datastore.OutgoingProject {
			endpoint.AdvancedSignatures = newEndpoint.AdvancedSignatures
		}

		if !util.IsStringEmpty(newEndpoint.HttpTimeout) {
			endpoint.HttpTimeout = newEndpoint.HttpTimeout
		}

		auth, err := ValidateEndpointAuthentication(newEndpoint.Authentication)
		if err != nil {
			return nil, err
		}

		endpoint.Authentication = auth

		endpoint.UpdatedAt = time.Now()

		err = endpointRepo.UpdateEndpoint(ctx, endpoint, project.UID)
		if err != nil {
			log.WithError(err).Error("failed to update endpoint")
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}

	case datastore.ErrEndpointNotFound:
		duration, err := time.ParseDuration(newEndpoint.RateLimitDuration)
		if err != nil {
			return nil, fmt.Errorf("an error occurred parsing the rate limit duration: %v", err)
		}

		endpoint = &datastore.Endpoint{
			UID:                ulid.Make().String(),
			ProjectID:          project.UID,
			OwnerID:            newEndpoint.OwnerID,
			Title:              newEndpoint.Name,
			SupportEmail:       newEndpoint.SupportEmail,
			SlackWebhookURL:    newEndpoint.SlackWebhookURL,
			TargetURL:          newEndpoint.URL,
			Description:        newEndpoint.Description,
			RateLimit:          newEndpoint.RateLimit,
			HttpTimeout:        newEndpoint.HttpTimeout,
			AdvancedSignatures: newEndpoint.AdvancedSignatures,
			AppID:              newEndpoint.AppID,
			RateLimitDuration:  duration.String(),
			Status:             datastore.ActiveEndpointStatus,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		if util.IsStringEmpty(endpoint.AppID) {
			endpoint.AppID = endpoint.UID
		}

		if util.IsStringEmpty(newEndpoint.Secret) {
			sc, err := util.GenerateSecret()
			if err != nil {
				return nil, &EndpointError{Err: err, delay: 10 * time.Second}
			}

			endpoint.Secrets = []datastore.Secret{
				{
					UID:       ulid.Make().String(),
					Value:     sc,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
		} else {
			endpoint.Secrets = append(endpoint.Secrets, datastore.Secret{
				UID:       ulid.Make().String(),
				Value:     newEndpoint.Secret,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
		}

		auth, err := ValidateEndpointAuthentication(endpoint.Authentication)
		if err != nil {
			return nil, err
		}

		endpoint.Authentication = auth
		err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
		if err != nil {
			log.WithError(err).Error("failed to create endpoint")
			return nil, &EndpointError{Err: err, delay: 10 * time.Second}
		}

		return endpoint, nil
	default:
		return nil, &EndpointError{Err: err, delay: 10 * time.Second}
	}

	return endpoint, nil
}

func ValidateEndpointAuthentication(auth *datastore.EndpointAuthentication) (*datastore.EndpointAuthentication, error) {
	if auth != nil && !util.IsStringEmpty(string(auth.Type)) {
		if err := util.Validate(auth); err != nil {
			return nil, err
		}

		if auth == nil && auth.Type == datastore.APIKeyAuthentication {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("api key field is required"))
		}

		return auth, nil
	}

	return nil, nil
}
