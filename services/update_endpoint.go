package services

import (
    "context"
    "time"

    "github.com/frain-dev/convoy"

    "github.com/frain-dev/convoy/internal/pkg/license"

    "github.com/frain-dev/convoy/pkg/log"

    "github.com/frain-dev/convoy/api/models"
    "github.com/frain-dev/convoy/cache"
    "github.com/frain-dev/convoy/datastore"
    "github.com/frain-dev/convoy/util"
)

type UpdateEndpointService struct {
    Cache        cache.Cache
    EndpointRepo datastore.EndpointRepository
    ProjectRepo  datastore.ProjectRepository
    Licenser     license.Licenser

    E        models.UpdateEndpoint
    Endpoint *datastore.Endpoint
    Project  *datastore.Project
}

func (a *UpdateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
    url, err := util.ValidateEndpoint(a.E.URL, a.Project.Config.SSL.EnforceSecureEndpoints)
    if err != nil {
        return nil, &ServiceError{ErrMsg: err.Error()}
    }

    a.E.URL = url

    endpoint := a.Endpoint

    endpoint, err = a.EndpointRepo.FindEndpointByID(ctx, endpoint.UID, a.Project.UID)
    if err != nil {
        return nil, &ServiceError{ErrMsg: err.Error()}
    }

    endpoint, err = a.updateEndpoint(endpoint, a.E, a.Project)
    if err != nil {
        return nil, &ServiceError{ErrMsg: err.Error()}
    }

    err = a.EndpointRepo.UpdateEndpoint(ctx, endpoint, endpoint.ProjectID)
    if err != nil {
        log.FromContext(ctx).WithError(err).Error("failed to update endpoint")
        return endpoint, &ServiceError{ErrMsg: "an error occurred while updating endpoints", Err: err}

    }

    return endpoint, nil
}

func (a *UpdateEndpointService) updateEndpoint(endpoint *datastore.Endpoint, e models.UpdateEndpoint, project *datastore.Project) (*datastore.Endpoint, error) {
    endpoint.Url = e.URL
    endpoint.Description = e.Description

    endpoint.Name = *e.Name

    if e.SupportEmail != nil && a.Licenser.AdvancedEndpointMgmt() {
        endpoint.SupportEmail = *e.SupportEmail
    }

    if e.SlackWebhookURL != nil && a.Licenser.AdvancedEndpointMgmt() {
        endpoint.SlackWebhookURL = *e.SlackWebhookURL
    }

    if e.RateLimit >= 0 {
        endpoint.RateLimit = e.RateLimit
    }

    endpoint.RateLimitDuration = e.RateLimitDuration

    if e.AdvancedSignatures != nil && project.Type == datastore.OutgoingProject {
        endpoint.AdvancedSignatures = *e.AdvancedSignatures
    }

    if e.HttpTimeout != 0 {
        endpoint.HttpTimeout = e.HttpTimeout

        if !a.Licenser.AdvancedEndpointMgmt() {
            // switch to default timeout
            endpoint.HttpTimeout = convoy.HTTP_TIMEOUT
        }
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
