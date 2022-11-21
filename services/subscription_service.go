package services

import (
	"context"
	"encoding/json"
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
	subRepo    datastore.SubscriptionRepository
	appRepo    datastore.ApplicationRepository
	sourceRepo datastore.SourceRepository
}

func NewSubscriptionService(subRepo datastore.SubscriptionRepository, appRepo datastore.ApplicationRepository, sourceRepo datastore.SourceRepository) *SubcriptionService {
	return &SubcriptionService{subRepo: subRepo, sourceRepo: sourceRepo, appRepo: appRepo}
}

func (s *SubcriptionService) CreateSubscription(ctx context.Context, group *datastore.Group, newSubscription *models.Subscription) (*datastore.Subscription, error) {
	if err := util.Validate(newSubscription); err != nil {
		log.WithError(err).Error(ErrValidateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	app, err := s.appRepo.FindApplicationByID(ctx, newSubscription.AppID)
	if err != nil {
		log.WithError(err).Error("failed to find application by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find application by id"))
	}

	if app.GroupID != group.UID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("app does not belong to group"))
	}

	_, err = findAppEndpoint(app.Endpoints, newSubscription.EndpointID)
	if err != nil {
		log.WithError(err).Error("failed to find app endpoint")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
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
		AppID:      newSubscription.AppID,
		SourceID:   newSubscription.SourceID,
		EndpointID: newSubscription.EndpointID,

		RetryConfig:     retryConfig,
		AlertConfig:     newSubscription.AlertConfig,
		FilterConfig:    newSubscription.FilterConfig,
		RateLimitConfig: newSubscription.RateLimitConfig,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),

		Status: datastore.ActiveSubscriptionStatus,
	}

	if newSubscription.DisableEndpoint != nil {
		subscription.DisableEndpoint = newSubscription.DisableEndpoint
	}

	if subscription.FilterConfig == nil {
		subscription.FilterConfig = &datastore.FilterConfiguration{}
	}

	if subscription.FilterConfig.EventTypes == nil || len(subscription.FilterConfig.EventTypes) == 0 {
		subscription.FilterConfig.EventTypes = []string{"*"}
	}

	if len(subscription.FilterConfig.Filter) == 0 {
		subscription.FilterConfig.Filter = map[string]interface{}{}
	} else {
		// validate that the filter is a json string
		_, err := json.Marshal(subscription.FilterConfig.Filter)
		if err != nil {
			log.WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
			return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidSubscriptionFilterFormat)
		}
	}

	err = s.subRepo.CreateSubscription(ctx, group.UID, subscription)
	if err != nil {
		log.WithError(err).Error(ErrCreateSubscriptionError.Error())
		return nil, util.NewServiceError(http.StatusBadRequest, ErrCreateSubscriptionError)
	}

	return subscription, nil
}

func findAppEndpoint(endpoints []datastore.Endpoint, id string) (*datastore.Endpoint, error) {
	for _, endpoint := range endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == nil {
			return &endpoint, nil
		}
	}
	return nil, datastore.ErrEndpointNotFound
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

	if update.FilterConfig != nil {
		if len(update.FilterConfig.EventTypes) > 0 {
			subscription.FilterConfig.EventTypes = update.FilterConfig.EventTypes
		}

		if len(update.FilterConfig.Filter) > 0 {
			// validate that the filter is a json string
			_, err := json.Marshal(update.FilterConfig.Filter)
			if err != nil {
				log.WithError(err).Error(ErrInvalidSubscriptionFilterFormat.Error())
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

func (s *SubcriptionService) TestSubscriptionFilter(ctx context.Context, testRequest map[string]interface{}, bodyFilter map[string]interface{}) (bool, error) {
	passed, err := s.subRepo.TestSubscriptionFilter(ctx, testRequest, bodyFilter)
	if err != nil {
		log.WithError(err).Error(ErrValidateSubscriptionFilterError.Error())
		return false, util.NewServiceError(http.StatusBadRequest, err)
	}

	return passed, nil
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
		endpoint, err := s.appRepo.FindApplicationEndpointByID(ctx, sub.AppID, sub.EndpointID)
		if err != nil {
			log.WithError(err).Error("failed to find subscription app endpoint")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription app endpoint"))
		}

		sub.Endpoint = endpoint
	}

	if sub.AppID != "" {
		app, err := s.appRepo.FindApplicationByID(ctx, sub.AppID)
		if err != nil {
			log.WithError(err).Error("failed to find subscription app ")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find subscription app"))
		}

		sub.App = app
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
