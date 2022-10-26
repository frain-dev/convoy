package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EndpointService struct {
	endpointRepo      datastore.EndpointRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	cache             cache.Cache
}

func NewEndpointService(endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache) *EndpointService {
	return &EndpointService{endpointRepo: endpointRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, cache: cache}
}

func (a *EndpointService) LoadEndpointsPaged(ctx context.Context, uid string, q string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	endpoints, paginationData, err := a.endpointRepo.LoadEndpointsPaged(ctx, uid, strings.TrimSpace(q), pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch endpoints")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching endpoints"))
	}

	return endpoints, paginationData, nil
}

func (a *EndpointService) CreateEndpoint(ctx context.Context, e models.Endpoint, groupID string) (*datastore.Endpoint, error) {
	if err := util.Validate(e); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	url, err := util.CleanEndpoint(e.URL)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	e.URL = url
	if e.RateLimit == 0 {
		e.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(e.RateLimitDuration) {
		e.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	duration, err := time.ParseDuration(e.RateLimitDuration)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("an error occurred parsing the rate limit duration: %v", err))
	}

	endpoint := &datastore.Endpoint{
		UID:               uuid.New().String(),
		GroupID:           groupID,
		Title:             e.Name,
		SupportEmail:      e.SupportEmail,
		SlackWebhookURL:   e.SlackWebhookURL,
		IsDisabled:        e.IsDisabled,
		TargetURL:         e.URL,
		Description:       e.Description,
		Secret:            e.Secret,
		RateLimit:         e.RateLimit,
		HttpTimeout:       e.HttpTimeout,
		RateLimitDuration: duration.String(),
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	if util.IsStringEmpty(e.Secret) {
		endpoint.Secret, err = util.GenerateSecret()
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf(fmt.Sprintf("could not generate secret...%v", err.Error())))
		}
	}

	auth, err := validateEndpointAuthentication(e.Authentication)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoint.Authentication = auth
	err = a.endpointRepo.CreateEndpoint(ctx, endpoint, groupID)
	if err != nil {
		log.WithError(err).Error("failed to create endpoint")
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("an error occurred while adding endpoint"))
	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(endpoint.UID).String()
	err = a.cache.Set(ctx, endpointCacheKey, &endpoint, time.Minute*5)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update endpoint cache"))
	}

	return endpoint, nil
}

func (a *EndpointService) UpdateEndpoint(ctx context.Context, e models.UpdateEndpoint, endpoint *datastore.Endpoint) (*datastore.Endpoint, error) {
	if err := util.Validate(e); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	url, err := util.CleanEndpoint(e.URL)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	e.URL = url

	endpoint, err = a.endpointRepo.FindEndpointByID(ctx, endpoint.UID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoint, err = updateEndpoint(endpoint, e)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, err)
	}

	err = a.endpointRepo.UpdateEndpoint(ctx, endpoint, endpoint.GroupID)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating endpoints"))
	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(endpoint.UID).String()
	err = a.cache.Set(ctx, endpointCacheKey, &endpoint, time.Minute*5)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return endpoint, nil
}

func (a *EndpointService) DeleteEndpoint(ctx context.Context, e *datastore.Endpoint, groupID string) error {

	err := a.endpointRepo.UpdateEndpoint(ctx, e, groupID)
	if err != nil {
		log.WithError(err).Error("failed to delete endpoint")
		return util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting endpoint"))
	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(e.UID).String()
	err = a.cache.Set(ctx, endpointCacheKey, &e, time.Minute*5)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update endpoint cache"))
	}

	return nil
}

func (a *EndpointService) CountGroupEndpoints(ctx context.Context, groupID string) (int64, error) {
	endpoints, err := a.endpointRepo.CountGroupEndpoints(ctx, groupID)
	if err != nil {
		log.WithError(err).Error("failed to count group endpoints")
		return 0, util.NewServiceError(http.StatusBadRequest, errors.New("failed to count group endpoints"))
	}

	return endpoints, nil
}

func updateEndpoint(endpoint *datastore.Endpoint, e models.UpdateEndpoint) (*datastore.Endpoint, error) {
	endpoint.TargetURL = e.URL
	endpoint.Description = e.Description

	endpoint.Title = *e.Name

	if e.SupportEmail != nil {
		endpoint.SupportEmail = *e.SupportEmail
	}

	if e.IsDisabled != nil {
		endpoint.IsDisabled = *e.IsDisabled
	}

	if e.SlackWebhookURL != nil {
		endpoint.SlackWebhookURL = *e.SlackWebhookURL
	}

	if e.RateLimit != 0 {
		endpoint.RateLimit = e.RateLimit
	}

	if !util.IsStringEmpty(e.RateLimitDuration) {
		duration, err := time.ParseDuration(e.RateLimitDuration)
		if err != nil {
			return nil, err
		}

		endpoint.RateLimitDuration = duration.String()
	}

	if !util.IsStringEmpty(e.HttpTimeout) {
		endpoint.HttpTimeout = e.HttpTimeout
	}

	if !util.IsStringEmpty(e.Secret) {
		endpoint.Secret = e.Secret
	}

	auth, err := validateEndpointAuthentication(e.Authentication)
	if err != nil {
		return nil, err
	}

	endpoint.Authentication = auth

	endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	return endpoint, nil
}

func validateEndpointAuthentication(auth *datastore.EndpointAuthentication) (*datastore.EndpointAuthentication, error) {
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
