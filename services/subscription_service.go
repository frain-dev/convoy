package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrSubscriptionNotFound         = errors.New("subscription not found")
	ErrUpateSubscriptionError       = errors.New("failed to update subscription")
	ErrCreateSubscriptionError      = errors.New("failed to create subscription")
	ErrDeletedSubscriptionError     = errors.New("failed to delete subscription")
	ErrValidateSubscriptionError    = errors.New("failed to validate subscription")
	ErrCannotFetchSubcriptionsError = errors.New("an error occurred while fetching subscriptions")
)

type SubcriptionService struct {
	subRepo      datastore.SubscriptionRepository
	endpointRepo datastore.EndpointRepository
	sourceRepo   datastore.SourceRepository
}

func NewSubscriptionService(subRepo datastore.SubscriptionRepository, endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository) *SubcriptionService {
	return &SubcriptionService{subRepo: subRepo, sourceRepo: sourceRepo, endpointRepo: endpointRepo}
}

func (s *SubcriptionService) CreateSubscription(ctx context.Context, group *datastore.Group, newSubscription *models.Subscription) (*datastore.Subscription, error) {
	if err := util.Validate(newSubscription); err != nil {
		log.WithError(err).Error(ErrValidateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoint, err := s.findEndpoint(ctx, newSubscription.AppID, newSubscription.EndpointID)
	if err != nil {
		log.WithError(err).Error("failed to find endpoint by id")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if endpoint.GroupID != group.UID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("endpoint does not belong to group"))
	}

	if group.Type == datastore.IncomingGroup {
		_, err = s.sourceRepo.FindSourceByID(ctx, group.UID, newSubscription.SourceID)
		if err != nil {
			log.WithError(err).Error("failed to find source by id")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find source by id"))
		}
	}

	retryConfig, err := getRetryConfig(newSubscription.RetryConfig)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	subscription := &datastore.Subscription{
		GroupID:    group.UID,
		UID:        uuid.New().String(),
		Name:       newSubscription.Name,
		Type:       datastore.SubscriptionTypeAPI,
		SourceID:   newSubscription.SourceID,
		EndpointID: newSubscription.EndpointID,

		RetryConfig:     retryConfig,
		AlertConfig:     newSubscription.AlertConfig,
		FilterConfig:    newSubscription.FilterConfig,
		RateLimitConfig: newSubscription.RateLimitConfig,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),

		Status:         datastore.ActiveSubscriptionStatus,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if newSubscription.DisableEndpoint != nil {
		subscription.DisableEndpoint = newSubscription.DisableEndpoint
	}

	if subscription.FilterConfig == nil ||
		subscription.FilterConfig.EventTypes == nil ||
		len(subscription.FilterConfig.EventTypes) == 0 {
		subscription.FilterConfig = &datastore.FilterConfiguration{EventTypes: []string{"*"}}
	}

	err = s.subRepo.CreateSubscription(ctx, group.UID, subscription)
	if err != nil {
		log.WithError(err).Error(ErrCreateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrCreateSubscriptionError)
	}

	return subscription, nil
}

func (s *SubcriptionService) UpdateSubscription(ctx context.Context, groupId string, subscriptionId string, update *models.UpdateSubscription) (*datastore.Subscription, error) {
	if err := util.Validate(update); err != nil {
		log.WithError(err).Error(ErrValidateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	subscription, err := s.subRepo.FindSubscriptionByID(ctx, groupId, subscriptionId)
	if err != nil {
		log.WithError(err).Error(ErrSubscriptionNotFound.Error())
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

	if update.FilterConfig != nil && len(update.FilterConfig.EventTypes) > 0 {
		subscription.FilterConfig.EventTypes = update.FilterConfig.EventTypes
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

	if update.DisableEndpoint != nil {
		subscription.DisableEndpoint = update.DisableEndpoint
	}

	err = s.subRepo.UpdateSubscription(ctx, groupId, subscription)
	if err != nil {
		log.WithError(err).Error(ErrUpateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrUpateSubscriptionError)
	}

	return subscription, nil
}

func (s *SubcriptionService) ToggleSubscriptionStatus(ctx context.Context, groupId string, subscriptionId string) (*datastore.Subscription, error) {
	subscription, err := s.subRepo.FindSubscriptionByID(ctx, groupId, subscriptionId)
	if err != nil {
		log.WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrSubscriptionNotFound)
	}

	switch subscription.Status {
	case datastore.ActiveSubscriptionStatus:
		subscription.Status = datastore.InactiveSubscriptionStatus
	case datastore.InactiveSubscriptionStatus:
		subscription.Status = datastore.ActiveSubscriptionStatus
	case datastore.PendingSubscriptionStatus:
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("subscription is in pending status"))
	default:
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("unknown subscription status: %s", subscription.Status))
	}

	err = s.subRepo.UpdateSubscriptionStatus(ctx, groupId, subscription.UID, subscription.Status)
	if err != nil {
		log.WithError(err).Error("failed to update subscription status")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update subscription status"))
	}

	return subscription, nil
}

func (s *SubcriptionService) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	err := s.subRepo.DeleteSubscription(ctx, groupId, subscription)
	if err != nil {
		log.WithError(err).Error(ErrDeletedSubscriptionError.Error())
		return util.NewServiceError(http.StatusBadRequest, ErrDeletedSubscriptionError)
	}

	return nil
}

func (s *SubcriptionService) FindSubscriptionByID(ctx context.Context, group *datastore.Group, subscriptionId string, skipCache bool) (*datastore.Subscription, error) {
	sub, err := s.subRepo.FindSubscriptionByID(ctx, group.UID, subscriptionId)
	if err != nil {
		log.WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, util.NewServiceError(http.StatusNotFound, ErrSubscriptionNotFound)
	}

	if skipCache {
		return sub, nil
	}

	// only incoming groups have sources
	if group.Type == datastore.IncomingGroup && sub.SourceID != "" {
		source, err := s.sourceRepo.FindSourceByID(ctx, group.UID, sub.SourceID)
		if err != nil {
			log.WithError(err).Error("failed to find subscription source")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription source"))
		}
		sub.Source = source
	}

	if sub.EndpointID != "" {
		endpoint, err := s.endpointRepo.FindEndpointByID(ctx, sub.EndpointID)
		if err != nil {
			log.WithError(err).Error("failed to find subscription app endpoint")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription app endpoint"))
		}

		sub.Endpoint = endpoint
	}

	return sub, nil
}

func (s *SubcriptionService) LoadSubscriptionsPaged(ctx context.Context, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	subscriptions, paginatedData, err := s.subRepo.LoadSubscriptionsPaged(ctx, filter.GroupID, filter, pageable)
	if err != nil {
		log.WithError(err).Error(ErrCannotFetchSubcriptionsError.Error())
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, ErrCannotFetchSubcriptionsError)
	}

	if subscriptions == nil {
		subscriptions = make([]datastore.Subscription, 0)
	}

	return subscriptions, paginatedData, nil
}

func (s *SubcriptionService) findEndpoint(ctx context.Context, appID, endpointID string) (*datastore.Endpoint, error) {
	if !util.IsStringEmpty(appID) {
		endpoints, err := s.endpointRepo.FindEndpointsByAppID(ctx, appID)

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

	endpoint, err := s.endpointRepo.FindEndpointByID(ctx, endpointID)
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
