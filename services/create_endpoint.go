package services

import (
	"context"
	"errors"
	// "fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/net"
	"net/http"
	"net/url"
	"time"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type CreateEndpointService struct {
	PortalLinkRepo datastore.PortalLinkRepository
	EndpointRepo   datastore.EndpointRepository
	ProjectRepo    datastore.ProjectRepository
	Licenser       license.Licenser
	FeatureFlag    *fflag.FFlag
	Logger         log.StdLogger
	E              models.CreateEndpoint
	ProjectID      string
}

func (a *CreateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	project, err := a.ProjectRepo.FetchProjectByID(ctx, a.ProjectID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to load endpoint project", Err: err}
	}

	endpointUrl, err := a.ValidateEndpoint(ctx, project.Config.SSL.EnforceSecureEndpoints)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	a.E.URL = endpointUrl

	truthValue := true
	switch project.Type {
	case datastore.IncomingProject:
		a.E.AdvancedSignatures = &truthValue
	case datastore.OutgoingProject:
		if a.E.AdvancedSignatures != nil {
			break
		}

		a.E.AdvancedSignatures = &truthValue
	}

	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          a.ProjectID,
		OwnerID:            a.E.OwnerID,
		Name:               a.E.Name,
		SupportEmail:       a.E.SupportEmail,
		SlackWebhookURL:    a.E.SlackWebhookURL,
		Url:                a.E.URL,
		Description:        a.E.Description,
		RateLimit:          a.E.RateLimit,
		HttpTimeout:        a.E.HttpTimeout,
		AdvancedSignatures: *a.E.AdvancedSignatures,
		AppID:              a.E.AppID,
		RateLimitDuration:  a.E.RateLimitDuration,
		Status:             datastore.ActiveEndpointStatus,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if !a.Licenser.AdvancedEndpointMgmt() {
		// switch to default timeout
		endpoint.HttpTimeout = convoy.HTTP_TIMEOUT

		endpoint.SupportEmail = ""
		endpoint.SlackWebhookURL = ""
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
		if errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailableUpgradeOrRevert) {
			return nil, &ServiceError{ErrMsg: err.Error(), Err: err}
		}
		return nil, &ServiceError{ErrMsg: "an error occurred while adding endpoint", Err: err}
	}

	return endpoint, nil
}

func (a *CreateEndpointService) ValidateEndpoint(ctx context.Context, enforceSecure bool) (string, error) {
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
		cfg, innerErr := config.Get()
		if innerErr != nil {
			return "", innerErr
		}

		caCertTLSCfg, innerErr := config.GetCaCert()
		if innerErr != nil {
			return "", innerErr
		}

		dispatcher, innerErr := net.NewDispatcher(
			a.Licenser,
			a.FeatureFlag,
			net.LoggerOption(a.Logger),
			net.ProxyOption(cfg.Server.HTTP.HttpProxy),
			net.AllowListOption(cfg.Dispatcher.AllowList),
			net.BlockListOption(cfg.Dispatcher.BlockList),
			net.TLSConfigOption(cfg.Dispatcher.InsecureSkipVerify, a.Licenser, caCertTLSCfg),
		)
		if innerErr != nil {
			return "", innerErr
		}

		// TODO: this does a GET, but the endpoint needs a POST!
		pingErr = dispatcher.Ping(ctx, a.E.URL, 10*time.Second)
		if pingErr != nil {
			// TODO: log this
			fmt.Errorf("failed to ping tls endpoint: %v", pingErr)
		}
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	return u.String(), nil
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
