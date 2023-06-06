package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type UpdateEndpointService struct {
	Cache        cache.Cache
	EndpointRepo datastore.EndpointRepository
	ProjectRepo  datastore.ProjectRepository

	E        models.UpdateEndpoint
	Endpoint *datastore.Endpoint
	Project  *datastore.Project
}

func (a *UpdateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	url, err := util.CleanEndpoint(a.E.URL)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	a.E.URL = url

	endpoint := a.Endpoint

	endpoint, err = a.EndpointRepo.FindEndpointByID(ctx, endpoint.UID, a.Project.UID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	endpoint, err = updateEndpoint(endpoint, a.E, a.Project)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	err = a.EndpointRepo.UpdateEndpoint(ctx, endpoint, endpoint.ProjectID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update endpoint")
		return endpoint, &ServiceError{ErrMsg: "an error occurred while updating endpoints", Err: err}

	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(endpoint.UID).String()
	err = a.Cache.Set(ctx, endpointCacheKey, &endpoint, time.Minute*5)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update endpoint cache")
		return endpoint, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return endpoint, nil
}

func updateEndpoint(endpoint *datastore.Endpoint, e models.UpdateEndpoint, project *datastore.Project) (*datastore.Endpoint, error) {
	endpoint.TargetURL = e.URL
	endpoint.Description = e.Description

	endpoint.Title = *e.Name

	if e.SupportEmail != nil {
		endpoint.SupportEmail = *e.SupportEmail
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

	if e.AdvancedSignatures != nil && project.Type == datastore.OutgoingProject {
		endpoint.AdvancedSignatures = *e.AdvancedSignatures
	}

	if !util.IsStringEmpty(e.HttpTimeout) {
		endpoint.HttpTimeout = e.HttpTimeout
	}

	if !util.IsStringEmpty(e.OwnerID) {
		endpoint.OwnerID = e.OwnerID
	}

	auth, err := ValidateEndpointAuthentication(e.Authentication.Transform())
	if err != nil {
		return nil, err
	}

	endpoint.Authentication = auth

	endpoint.UpdatedAt = time.Now()

	return endpoint, nil
}
