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
	ErrValidateSubscriptionError    = errors.New("failed to validate group update")
	ErrCannotFetchSubcriptionsError = errors.New("an error occurred while fetching subscriptions")
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

	subscription := &datastore.Subscription{
		GroupID:    group.UID,
		UID:        uuid.New().String(),
		Name:       newSubscription.Name,
		Type:       newSubscription.Type,
		AppID:      newSubscription.AppID,
		SourceID:   newSubscription.SourceID,
		EndpointID: newSubscription.EndpointID,

		RetryConfig:  newSubscription.RetryConfig,
		AlertConfig:  newSubscription.AlertConfig,
		FilterConfig: newSubscription.FilterConfig,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),

		Status:         datastore.ActiveSubscriptionStatus,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if subscription.FilterConfig == nil ||
		subscription.FilterConfig.EventTypes == nil ||
		len(subscription.FilterConfig.EventTypes) == 0 {
		subscription.FilterConfig = &datastore.FilterConfiguration{EventTypes: []string{"*"}}
	}

	if subscription.AlertConfig == nil {
		subscription.AlertConfig = &datastore.DefaultAlertConfig
	}

	if subscription.RetryConfig == nil {
		subscription.RetryConfig = &datastore.DefaultRetryConfig
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
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
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
		subscription.AlertConfig.Count = update.AlertConfig.Count
	}

	if update.AlertConfig != nil && !util.IsStringEmpty(update.AlertConfig.Threshold) {
		subscription.AlertConfig.Threshold = update.AlertConfig.Threshold
	}

	if update.RetryConfig != nil && !util.IsStringEmpty(string(update.RetryConfig.Type)) {
		subscription.RetryConfig.Type = update.RetryConfig.Type
	}

	if update.RetryConfig != nil && !util.IsStringEmpty(update.RetryConfig.Duration) {
		subscription.RetryConfig.Duration = update.RetryConfig.Duration
	}

	if update.RetryConfig != nil && update.RetryConfig.RetryCount > 0 {
		subscription.RetryConfig.RetryCount = update.RetryConfig.RetryCount
	}

	if update.FilterConfig != nil && len(update.FilterConfig.EventTypes) > 0 {
		subscription.FilterConfig.EventTypes = update.FilterConfig.EventTypes
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

func (s *SubcriptionService) LoadSubscriptionsPaged(ctx context.Context, groupId string, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	var subscriptions []datastore.Subscription
	var paginatedData datastore.PaginationData
	subscriptions, paginatedData, err := s.subRepo.LoadSubscriptionsPaged(ctx, groupId, pageable)
	if err != nil {
		log.WithError(err).Error(ErrCannotFetchSubcriptionsError.Error())
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, ErrCannotFetchSubcriptionsError)
	}

	if subscriptions == nil {
		subscriptions = make([]datastore.Subscription, 0)
	}

	appMap := datastore.AppMap{}
	sourceMap := datastore.SourceMap{}
	endpointMap := datastore.EndpointMap{}

	for i, sub := range subscriptions {
		if _, ok := appMap[sub.AppID]; !ok {
			a, err := s.appRepo.FindApplicationByID(ctx, sub.AppID)
			if err == nil {
				aa := &datastore.Application{
					UID:          a.UID,
					Title:        a.Title,
					GroupID:      a.GroupID,
					SupportEmail: a.SupportEmail,
				}
				appMap[sub.AppID] = aa
			}
		}

		if _, ok := sourceMap[sub.SourceID]; !ok {
			ev, err := s.sourceRepo.FindSourceByID(ctx, sub.GroupID, sub.SourceID)
			if err == nil {
				source := &datastore.Source{
					UID:        ev.UID,
					Name:       ev.Name,
					Type:       ev.Type,
					Verifier:   ev.Verifier,
					GroupID:    ev.GroupID,
					MaskID:     ev.MaskID,
					IsDisabled: ev.IsDisabled,
				}
				sourceMap[sub.SourceID] = source
			}
		}

		if _, ok := endpointMap[sub.EndpointID]; !ok {
			en, err := s.appRepo.FindApplicationEndpointByID(ctx, sub.AppID, sub.EndpointID)
			if err == nil {
				endpoint := &datastore.Endpoint{
					UID:               en.UID,
					TargetURL:         en.TargetURL,
					DocumentStatus:    en.DocumentStatus,
					Secret:            en.Secret,
					HttpTimeout:       en.HttpTimeout,
					RateLimit:         en.RateLimit,
					RateLimitDuration: en.RateLimitDuration,
				}
				endpointMap[sub.EndpointID] = endpoint
			}
		}

		subscriptions[i].App = appMap[sub.AppID]
		subscriptions[i].Source = sourceMap[sub.SourceID]
		subscriptions[i].Endpoint = endpointMap[sub.EndpointID]
	}

	return subscriptions, paginatedData, nil
}
