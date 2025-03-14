package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrCantUseEndpointForTwoSubs = errors.New("can't use an endpoint for two subscriptions")
	ErrValidateSubscriptionError = errors.New("failed to validate subscription")
	ErrUpdateSubscriptionError   = errors.New("failed to update subscription")
)

type UpdateSubscriptionService struct {
	SubRepo      datastore.SubscriptionRepository
	EndpointRepo datastore.EndpointRepository
	ProjectRepo  datastore.ProjectRepository
	SourceRepo   datastore.SourceRepository
	Licenser     license.Licenser

	ProjectId      string
	SubscriptionId string
	Update         *models.UpdateSubscription
}

func (s *UpdateSubscriptionService) Run(ctx context.Context) (*datastore.Subscription, error) {
	subscription, err := s.SubRepo.FindSubscriptionByID(ctx, s.ProjectId, s.SubscriptionId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find subscription")
		return nil, &ServiceError{ErrMsg: "failed to find subscription", Err: err}
	}

	if !util.IsStringEmpty(s.Update.EndpointID) {
		subscription.EndpointID = s.Update.EndpointID
	}

	project, err := s.findProject(ctx, s.ProjectId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find project by id")
		return nil, &ServiceError{ErrMsg: "failed to find project by id", Err: err}
	}

	endpoint, err := s.findEndpoint(ctx, s.Update.EndpointID, s.ProjectId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoint by id")
		return nil, &ServiceError{ErrMsg: "failed to find endpoint by id: the endpoint may not belong to project", Err: err}
	}

	if project.Type == datastore.IncomingProject {
		_, err = s.SourceRepo.FindSourceByID(ctx, project.UID, s.Update.SourceID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find source by id")
			return nil, &ServiceError{ErrMsg: "failed to find source by id"}
		}
	}

	if project.Type == datastore.OutgoingProject {
		count, err2 := s.SubRepo.CountEndpointSubscriptions(ctx, project.UID, endpoint.UID)
		if err2 != nil {
			log.FromContext(ctx).WithError(err2).Error("failed to count endpoint subscriptions")
			return nil, &ServiceError{ErrMsg: "failed to count endpoint subscriptions", Err: err2}
		}

		if count > 0 {
			return nil, &ServiceError{ErrMsg: "a subscription for this endpoint already exists"}
		}
	}

	retryConfig, err := s.Update.RetryConfig.Transform()
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !util.IsStringEmpty(s.Update.Name) {
		subscription.Name = s.Update.Name
	}

	if !util.IsStringEmpty(s.Update.SourceID) {
		subscription.SourceID = s.Update.SourceID
	}

	if !util.IsStringEmpty(s.Update.Function) && s.Licenser.Transformations() {
		subscription.Function = null.StringFrom(s.Update.Function)
	}

	if s.Update.AlertConfig != nil && s.Update.AlertConfig.Count > 0 {
		if subscription.AlertConfig == nil {
			subscription.AlertConfig = &datastore.AlertConfiguration{}
		}

		subscription.AlertConfig.Count = s.Update.AlertConfig.Count
	}

	if s.Update.AlertConfig != nil && !util.IsStringEmpty(s.Update.AlertConfig.Threshold) {
		if subscription.AlertConfig == nil {
			subscription.AlertConfig = &datastore.AlertConfiguration{}
		}

		subscription.AlertConfig.Threshold = s.Update.AlertConfig.Threshold
	}

	if s.Update.RetryConfig != nil && !util.IsStringEmpty(string(s.Update.RetryConfig.Type)) {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.Type = s.Update.RetryConfig.Type
	}

	if s.Update.RetryConfig != nil && !util.IsStringEmpty(s.Update.RetryConfig.Duration) {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.Duration = retryConfig.Duration
	}

	if s.Update.RetryConfig != nil && s.Update.RetryConfig.IntervalSeconds > 0 {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.RetryCount = retryConfig.RetryCount
	}

	if s.Update.RetryConfig != nil && s.Update.RetryConfig.RetryCount > 0 {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.RetryCount = s.Update.RetryConfig.RetryCount
	}

	if s.Update.FilterConfig != nil && s.Licenser.AdvancedSubscriptions() {
		if len(s.Update.FilterConfig.EventTypes) > 0 {
			subscription.FilterConfig.EventTypes = s.Update.FilterConfig.EventTypes
		}

		if len(s.Update.FilterConfig.Filter.Body) > 0 || len(s.Update.FilterConfig.Filter.Headers) > 0 {
			// validate that the filter is a json string
			_, err = json.Marshal(s.Update.FilterConfig.Filter)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
				return nil, &ServiceError{ErrMsg: ErrInvalidSubscriptionFilterFormat.Error(), Err: err}
			}
			subscription.FilterConfig.Filter = s.Update.FilterConfig.Filter.Transform()
		} else {
			subscription.FilterConfig.Filter = datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}}
		}
	}

	if s.Update.RateLimitConfig != nil && s.Update.RateLimitConfig.Count > 0 {
		if subscription.RateLimitConfig == nil {
			subscription.RateLimitConfig = &datastore.RateLimitConfiguration{}
		}
		subscription.RateLimitConfig.Count = s.Update.RateLimitConfig.Count
	}

	if s.Update.RateLimitConfig != nil && s.Update.RateLimitConfig.Duration > 0 {
		if subscription.RateLimitConfig == nil {
			subscription.RateLimitConfig = &datastore.RateLimitConfiguration{}
		}
		subscription.RateLimitConfig.Duration = s.Update.RateLimitConfig.Duration
	}

	err = s.SubRepo.UpdateSubscription(ctx, s.ProjectId, subscription)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrUpdateSubscriptionError.Error())
		if strings.Contains(err.Error(), "key value violates unique constraint") {
			return nil, &ServiceError{ErrMsg: ErrCantUseEndpointForTwoSubs.Error(), Err: err}
		}
		return nil, &ServiceError{ErrMsg: ErrUpdateSubscriptionError.Error(), Err: err}
	}

	return subscription, nil
}

func (s *UpdateSubscriptionService) findProject(ctx context.Context, projectId string) (*datastore.Project, error) {
	project, err := s.ProjectRepo.FetchProjectByID(ctx, projectId)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (s *UpdateSubscriptionService) findEndpoint(ctx context.Context, endpointId, projectId string) (*datastore.Endpoint, error) {
	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, endpointId, projectId)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}
