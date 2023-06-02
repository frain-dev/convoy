package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type CreateEndpointService struct {
	Cache        cache.Cache
	EndpointRepo datastore.EndpointRepository
	ProjectRepo  datastore.ProjectRepository

	E         models.CreateEndpoint
    ProjectID string
}

func (a *CreateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	url, err := util.CleanEndpoint(a.E.URL)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	a.E.URL = url
	if a.E.RateLimit == 0 {
		a.E.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(a.E.RateLimitDuration) {
		a.E.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	duration, err := time.ParseDuration(a.E.RateLimitDuration)
	if err != nil {
		return nil, &ServiceError{ErrMsg: fmt.Sprintf("an error occurred parsing the rate limit duration: %v", err)}
	}

	project, err := a.ProjectRepo.FetchProjectByID(ctx, a.ProjectID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to load endpoint project", Err: err}
	}

	if project.Type == datastore.IncomingProject {
		a.E.AdvancedSignatures = true
	}

	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          a.ProjectID,
		OwnerID:            a.E.OwnerID,
		Title:              a.E.Name,
		SupportEmail:       a.E.SupportEmail,
		SlackWebhookURL:    a.E.SlackWebhookURL,
		TargetURL:          a.E.URL,
		Description:        a.E.Description,
		RateLimit:          a.E.RateLimit,
		HttpTimeout:        a.E.HttpTimeout,
		AdvancedSignatures: a.E.AdvancedSignatures,
		AppID:              a.E.AppID,
		RateLimitDuration:  duration.String(),
		Status:             datastore.ActiveEndpointStatus,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if util.IsStringEmpty(endpoint.AppID) {
		endpoint.AppID = endpoint.UID
	}

	if util.IsStringEmpty(a.E.Secret) {
		sc, err := util.GenerateSecret()
		if err != nil {
			return nil, &ServiceError{ErrMsg: "could not generate secret", Err: err}
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
			Value:     a.E.Secret,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

    auth, err := ValidateEndpointAuthentication(a.E.Authentication.Transform())
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	endpoint.Authentication = auth
	err = a.EndpointRepo.CreateEndpoint(ctx, endpoint, a.ProjectID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create endpoint")
		return nil, &ServiceError{ErrMsg: "an error occurred while adding endpoint", Err: err}
	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(endpoint.UID).String()
	err = a.Cache.Set(ctx, endpointCacheKey, &endpoint, time.Minute*5)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to update endpoint cache", Err: nil}
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
