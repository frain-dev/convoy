package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrSubscriptionNotFound            = errors.New("subscription not found")
	ErrUpateSubscriptionError          = errors.New("failed to update subscription")
	ErrCreateSubscriptionError         = errors.New("failed to create subscription")
	ErrDeletedSubscriptionError        = errors.New("failed to delete subscription")
	ErrValidateSubscriptionError       = errors.New("failed to validate subscription")
	ErrInvalidSubscriptionFilterFormat = errors.New("invalid subscription filter format")
	ErrValidateSubscriptionFilterError = errors.New("failed to validate subscription filter")
	ErrCannotFetchSubcriptionsError    = errors.New("an error occurred while fetching subscriptions")
)

type SubcriptionService struct {
	subRepo      datastore.SubscriptionRepository
	endpointRepo datastore.EndpointRepository
	sourceRepo   datastore.SourceRepository
}

func NewSubscriptionService(subRepo datastore.SubscriptionRepository, endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository) *SubcriptionService {
	return &SubcriptionService{subRepo: subRepo, sourceRepo: sourceRepo, endpointRepo: endpointRepo}
}

func (s *SubcriptionService) CreateSubscription(ctx context.Context, project *datastore.Project, newSubscription *models.Subscription) (*datastore.Subscription, error) {
	if err := util.Validate(newSubscription); err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrValidateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoint, err := s.findEndpoint(ctx, newSubscription.AppID, newSubscription.EndpointID, project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoint by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find endpoint by id"))
	}

	if endpoint.ProjectID != project.UID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("endpoint does not belong to project"))
	}

	if project.Type == datastore.IncomingProject {
		_, err = s.sourceRepo.FindSourceByID(ctx, project.UID, newSubscription.SourceID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find source by id")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find source by id"))
		}
	}

	if project.Type == datastore.OutgoingProject {
		count, err := s.subRepo.CountEndpointSubscriptions(ctx, project.UID, endpoint.UID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to count endpoint subscriptions")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to count endpoint subscriptions"))
		}

		if count > 0 {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("a subscription for this endpoint already exists"))
		}
	}

	retryConfig, err := getRetryConfig(newSubscription.RetryConfig)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		Name:       newSubscription.Name,
		Type:       datastore.SubscriptionTypeAPI,
		SourceID:   newSubscription.SourceID,
		EndpointID: newSubscription.EndpointID,

		RetryConfig:     retryConfig,
		AlertConfig:     newSubscription.AlertConfig,
		FilterConfig:    newSubscription.FilterConfig,
		RateLimitConfig: newSubscription.RateLimitConfig,

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if subscription.FilterConfig == nil {
		subscription.FilterConfig = &datastore.FilterConfiguration{}
	}

	if subscription.FilterConfig.EventTypes == nil || len(subscription.FilterConfig.EventTypes) == 0 {
		subscription.FilterConfig.EventTypes = []string{"*"}
	}

	if len(subscription.FilterConfig.Filter.Body) == 0 && len(subscription.FilterConfig.Filter.Headers) == 0 {
		subscription.FilterConfig.Filter = datastore.FilterSchema{
			Headers: datastore.M{},
			Body:    datastore.M{},
		}
	} else {
		// validate that the filter is a json string
		_, err := json.Marshal(subscription.FilterConfig.Filter)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
			return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidSubscriptionFilterFormat)
		}
	}

	err = s.subRepo.CreateSubscription(ctx, project.UID, subscription)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrCreateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrCreateSubscriptionError)
	}

	return subscription, nil
}

func (s *SubcriptionService) UpdateSubscription(ctx context.Context, projectId string, subscriptionId string, update *models.UpdateSubscription) (*datastore.Subscription, error) {
	if err := util.Validate(update); err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrValidateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	subscription, err := s.subRepo.FindSubscriptionByID(ctx, projectId, subscriptionId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrSubscriptionNotFound)
	}

	retryConfig, err := getRetryConfig(update.RetryConfig)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if !util.IsStringEmpty(update.Name) {
		subscription.Name = update.Name
	}

	if !util.IsStringEmpty(update.SourceID) {
		subscription.SourceID = update.SourceID
	}

	if !util.IsStringEmpty(update.EndpointID) {
		subscription.EndpointID = update.EndpointID
	}

	if update.AlertConfig != nil && update.AlertConfig.Count > 0 {
		if subscription.AlertConfig == nil {
			subscription.AlertConfig = &datastore.AlertConfiguration{}
		}

		subscription.AlertConfig.Count = update.AlertConfig.Count
	}

	if update.AlertConfig != nil && !util.IsStringEmpty(update.AlertConfig.Threshold) {
		if subscription.AlertConfig == nil {
			subscription.AlertConfig = &datastore.AlertConfiguration{}
		}

		subscription.AlertConfig.Threshold = update.AlertConfig.Threshold
	}

	if update.RetryConfig != nil && !util.IsStringEmpty(string(update.RetryConfig.Type)) {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.Type = update.RetryConfig.Type
	}

	if update.RetryConfig != nil && !util.IsStringEmpty(update.RetryConfig.Duration) {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.Duration = retryConfig.Duration
	}

	if update.RetryConfig != nil && update.RetryConfig.IntervalSeconds > 0 {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.RetryCount = retryConfig.RetryCount
	}

	if update.RetryConfig != nil && update.RetryConfig.RetryCount > 0 {
		if subscription.RetryConfig == nil {
			subscription.RetryConfig = &datastore.RetryConfiguration{}
		}

		subscription.RetryConfig.RetryCount = update.RetryConfig.RetryCount
	}

	if update.FilterConfig != nil {
		if len(update.FilterConfig.EventTypes) > 0 {
			subscription.FilterConfig.EventTypes = update.FilterConfig.EventTypes
		}

		if len(update.FilterConfig.Filter.Body) > 0 || len(update.FilterConfig.Filter.Headers) > 0 {
			// validate that the filter is a json string
			_, err := json.Marshal(update.FilterConfig.Filter)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
				return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidSubscriptionFilterFormat)
			}
			subscription.FilterConfig.Filter = update.FilterConfig.Filter
		}
	}

	if update.RateLimitConfig != nil && update.RateLimitConfig.Count > 0 {
		if subscription.RateLimitConfig == nil {
			subscription.RateLimitConfig = &datastore.RateLimitConfiguration{}
		}
		subscription.RateLimitConfig.Count = update.RateLimitConfig.Count
	}

	if update.RateLimitConfig != nil && update.RateLimitConfig.Duration > 0 {
		if subscription.RateLimitConfig == nil {
			subscription.RateLimitConfig = &datastore.RateLimitConfiguration{}
		}
		subscription.RateLimitConfig.Duration = update.RateLimitConfig.Duration
	}

	err = s.subRepo.UpdateSubscription(ctx, projectId, subscription)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrUpateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrUpateSubscriptionError)
	}

	return subscription, nil
}

func (s *SubcriptionService) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	err := s.subRepo.DeleteSubscription(ctx, groupId, subscription)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrDeletedSubscriptionError.Error())
		return util.NewServiceError(http.StatusBadRequest, ErrDeletedSubscriptionError)
	}

	return nil
}

func (s *SubcriptionService) TestSubscriptionFilter(ctx context.Context, testRequest map[string]interface{}, filter map[string]interface{}) (bool, error) {
	passed, err := s.subRepo.TestSubscriptionFilter(ctx, testRequest, filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrValidateSubscriptionFilterError.Error())
		return false, util.NewServiceError(http.StatusBadRequest, err)
	}

	return passed, nil
}

func (s *SubcriptionService) FindSubscriptionByID(ctx context.Context, project *datastore.Project, subscriptionId string, skipCache bool) (*datastore.Subscription, error) {
	sub, err := s.subRepo.FindSubscriptionByID(ctx, project.UID, subscriptionId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, util.NewServiceError(http.StatusNotFound, ErrSubscriptionNotFound)
	}

	if skipCache {
		return sub, nil
	}

	// only incoming projects have sources
	if project.Type == datastore.IncomingProject && sub.SourceID != "" {
		source, err := s.sourceRepo.FindSourceByID(ctx, project.UID, sub.SourceID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find subscription source")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription source"))
		}
		sub.Source = source
	}

	if sub.EndpointID != "" {
		endpoint, err := s.endpointRepo.FindEndpointByID(ctx, sub.EndpointID, project.UID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find subscription app endpoint")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription app endpoint"))
		}

		sub.Endpoint = endpoint
	}

	return sub, nil
}

func (s *SubcriptionService) LoadSubscriptionsPaged(ctx context.Context, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	subscriptions, paginatedData, err := s.subRepo.LoadSubscriptionsPaged(ctx, filter.ProjectID, filter, pageable)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrCannotFetchSubcriptionsError.Error())
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, ErrCannotFetchSubcriptionsError)
	}

	if subscriptions == nil {
		subscriptions = make([]datastore.Subscription, 0)
	}

	return subscriptions, paginatedData, nil
}

func (s *SubcriptionService) findEndpoint(ctx context.Context, appID, endpointID string, project *datastore.Project) (*datastore.Endpoint, error) {
	if !util.IsStringEmpty(appID) {
		endpoints, err := s.endpointRepo.FindEndpointsByAppID(ctx, appID, project.UID)
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

	endpoint, err := s.endpointRepo.FindEndpointByID(ctx, endpointID, project.UID)
	if err != nil {
		return nil, datastore.ErrEndpointNotFound
	}

	return endpoint, nil
}

func getRetryConfig(cfg *models.RetryConfiguration) (*datastore.RetryConfiguration, error) {
	if cfg == nil {
		return nil, nil
	}

	strategyConfig := &datastore.RetryConfiguration{Type: cfg.Type, RetryCount: cfg.RetryCount}
	if !util.IsStringEmpty(cfg.Duration) {
		interval, err := time.ParseDuration(cfg.Duration)
		if err != nil {
			return nil, err
		}

		strategyConfig.Duration = uint64(interval.Seconds())
		return strategyConfig, nil
	}

	strategyConfig.Duration = cfg.IntervalSeconds
	return strategyConfig, nil
}
