package services

import (
	"context"
	"errors"
	// "fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	// "github.com/frain-dev/convoy/net"
	"net/url"
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
	FeatureFlag  *fflag.FFlag
	Logger       log.StdLogger
	E            models.UpdateEndpoint
	Endpoint     *datastore.Endpoint
	Project      *datastore.Project
}

func (a *UpdateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	endpointUrl, err := a.ValidateEndpoint(ctx, a.Project.Config.SSL.EnforceSecureEndpoints)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	a.E.URL = endpointUrl

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

func (a *UpdateEndpointService) ValidateEndpoint(ctx context.Context, enforceSecure bool) (string, error) {
	if util.IsStringEmpty(a.E.URL) {
		return "", errors.New("please provide the endpoint url")
	}

	u, pingErr := url.Parse(a.E.URL)
	if pingErr != nil {
		return "", pingErr
	}

	switch u.Scheme {
	case "http":
		if enforceSecure {
			return "", errors.New("only https endpoints allowed")
		}
	case "https":
		// // TODO: this does a GET, but the endpoint needs a POST!
		// cfg, innerErr := config.Get()
		// if innerErr != nil {
		// 	return "", innerErr
		// }
		//
		// caCertTLSCfg, innerErr := config.GetCaCert()
		// if innerErr != nil {
		// 	return "", innerErr
		// }
		//
		// dispatcher, innerErr := net.NewDispatcher(
		// 	a.Licenser,
		// 	a.FeatureFlag,
		// 	net.LoggerOption(a.Logger),
		// 	net.ProxyOption(cfg.Server.HTTP.HttpProxy),
		// 	net.AllowListOption(cfg.Dispatcher.AllowList),
		// 	net.BlockListOption(cfg.Dispatcher.BlockList),
		// 	net.TLSConfigOption(cfg.Dispatcher.InsecureSkipVerify, a.Licenser, caCertTLSCfg),
		// )
		// if innerErr != nil {
		// 	return "", innerErr
		// }
		//
		// pingErr = dispatcher.Ping(ctx, a.E.URL, 10*time.Second)
		// if pingErr != nil {
		// 	return "", fmt.Errorf("failed to ping tls endpoint: %v", pingErr)
		// }
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	return u.String(), nil
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
