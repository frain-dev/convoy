package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrUpdateSubscriptionError   = errors.New("failed to update subscription")
	ErrValidateSubscriptionError = errors.New("failed to validate subscription")
)

type UpdateSubscriptionService struct {
	SubRepo      datastore.SubscriptionRepository
	EndpointRepo datastore.EndpointRepository
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

	if !util.IsStringEmpty(s.Update.EndpointID) {
		subscription.EndpointID = s.Update.EndpointID
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
			_, err := json.Marshal(s.Update.FilterConfig.Filter)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
				return nil, &ServiceError{ErrMsg: ErrInvalidSubscriptionFilterFormat.Error(), Err: err}
			}
			subscription.FilterConfig.Filter = s.Update.FilterConfig.Filter.Transform()
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
		return nil, &ServiceError{ErrMsg: ErrUpdateSubscriptionError.Error(), Err: err}
	}

	return subscription, nil
}
