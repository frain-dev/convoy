package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"gopkg.in/guregu/null.v4"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrInvalidSubscriptionFilterFormat = errors.New("invalid subscription filter format")
	ErrCreateSubscriptionError         = errors.New("failed to create subscription")
)

type CreateSubscriptionService struct {
	SubRepo         datastore.SubscriptionRepository
	EndpointRepo    datastore.EndpointRepository
	SourceRepo      datastore.SourceRepository
	Project         *datastore.Project
	NewSubscription *models.CreateSubscription
	Licenser        license.Licenser
}

func (s *CreateSubscriptionService) Run(ctx context.Context) (*datastore.Subscription, error) {
	endpoint, err := s.findEndpoint(ctx, s.NewSubscription.AppID, s.NewSubscription.EndpointID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoint by id")
		return nil, &ServiceError{ErrMsg: "failed to find endpoint by id", Err: err}
	}

	if endpoint.ProjectID != s.Project.UID {
		return nil, &ServiceError{ErrMsg: "endpoint does not belong to project"}
	}

	if s.Project.Type == datastore.IncomingProject {
		_, err = s.SourceRepo.FindSourceByID(ctx, s.Project.UID, s.NewSubscription.SourceID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find source by id")
			return nil, &ServiceError{ErrMsg: "failed to find source by id"}
		}
	}

	if s.Project.Type == datastore.OutgoingProject && !s.Project.Config.MultipleEndpointSubscriptions {
		count, err := s.SubRepo.CountEndpointSubscriptions(ctx, s.Project.UID, endpoint.UID, "")
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to count endpoint subscriptions")
			return nil, &ServiceError{ErrMsg: "failed to count endpoint subscriptions"}
		}

		if count > 0 {
			return nil, &ServiceError{ErrMsg: "a subscription for this endpoint already exists"}
		}
	}

	subscription := &datastore.Subscription{
		UID:          ulid.Make().String(),
		ProjectID:    s.Project.UID,
		Name:         s.NewSubscription.Name,
		Type:         datastore.SubscriptionTypeAPI,
		SourceID:     s.NewSubscription.SourceID,
		EndpointID:   s.NewSubscription.EndpointID,
		DeliveryMode: s.NewSubscription.DeliveryMode,

		AlertConfig:     s.NewSubscription.AlertConfig.Transform(),
		RateLimitConfig: s.NewSubscription.RateLimitConfig.Transform(),

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set default delivery mode if empty
	if subscription.DeliveryMode == "" {
		subscription.DeliveryMode = datastore.AtLeastOnceDeliveryMode
	} else if subscription.DeliveryMode != datastore.AtLeastOnceDeliveryMode && subscription.DeliveryMode != datastore.AtMostOnceDeliveryMode {
		return nil, &ServiceError{ErrMsg: "invalid delivery mode value, must be either 'at_least_once' or 'at_most_once'"}
	}

	if s.Licenser.AdvancedSubscriptions() {
		subscription.FilterConfig = s.NewSubscription.FilterConfig.Transform()
	}

	if s.Licenser.Transformations() {
		subscription.Function = null.StringFrom(s.NewSubscription.Function)
	}

	if subscription.FilterConfig == nil {
		subscription.FilterConfig = &datastore.FilterConfiguration{}
	}

	if len(subscription.FilterConfig.EventTypes) == 0 {
		subscription.FilterConfig.EventTypes = []string{"*"}
	}

	if len(subscription.FilterConfig.Filter.Body) == 0 && len(subscription.FilterConfig.Filter.Headers) == 0 {
		subscription.FilterConfig.Filter = datastore.FilterSchema{
			Headers:    datastore.M{},
			Body:       datastore.M{},
			RawHeaders: datastore.M{},
			RawBody:    datastore.M{},
		}
	} else {
		// validate that the filter is a json string
		_, err = json.Marshal(subscription.FilterConfig.Filter)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
			return nil, &ServiceError{ErrMsg: ErrInvalidSubscriptionFilterFormat.Error()}
		}
	}

	err = s.SubRepo.CreateSubscription(ctx, s.Project.UID, subscription)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrCreateSubscriptionError.Error())
		return nil, &ServiceError{ErrMsg: ErrCreateSubscriptionError.Error()}
	}

	return subscription, nil
}

func (s *CreateSubscriptionService) findEndpoint(ctx context.Context, appID, endpointID string) (*datastore.Endpoint, error) {
	if !util.IsStringEmpty(appID) {
		endpoints, err := s.EndpointRepo.FindEndpointsByAppID(ctx, appID, s.Project.UID)
		if err != nil {
			return nil, err
		}

		if len(endpoints) == 0 {
			return nil, errors.New("failed to find application by id")
		}

		for _, endpoint := range endpoints {
			if endpoint.UID == endpointID {
				return &endpoint, nil
			}
		}

		return nil, datastore.ErrEndpointNotFound
	}

	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, endpointID, s.Project.UID)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}
