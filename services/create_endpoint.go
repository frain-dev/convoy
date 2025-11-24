package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type CreateEndpointService struct {
	PortalLinkRepo     datastore.PortalLinkRepository
	EndpointRepo       datastore.EndpointRepository
	ProjectRepo        datastore.ProjectRepository
	Licenser           license.Licenser
	FeatureFlag        *fflag.FFlag
	FeatureFlagFetcher fflag.FeatureFlagFetcher
	DB                 database.Database
	Logger             log.StdLogger
	E                  models.CreateEndpoint
	ProjectID          string
}

func (a *CreateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	project, err := a.ProjectRepo.FetchProjectByID(ctx, a.ProjectID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to load endpoint project", Err: err}
	}

	endpointUrl, err := a.ValidateEndpoint(ctx, project.Config.SSL.EnforceSecureEndpoints, a.E.MtlsClientCert)
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
		ContentType:        a.E.ContentType,
		Status:             datastore.ActiveEndpointStatus,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if !a.Licenser.AdvancedEndpointMgmt() {
		// switch to the default timeout
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

	// Set mTLS client certificate if provided
	if a.E.MtlsClientCert != nil {
		// Check license before allowing mTLS configuration
		if !a.Licenser.MutualTLS() {
			return nil, &ServiceError{ErrMsg: ErrMutualTLSFeatureUnavailable}
		}

		// Validate both fields provided together
		mtlsEnabled := a.FeatureFlag.CanAccessOrgFeature(ctx, fflag.MTLS, a.FeatureFlagFetcher, project.OrganisationID)
		if !mtlsEnabled {
			log.FromContext(ctx).Warn("mTLS configuration provided but feature flag not enabled, ignoring mTLS config")
		} else {
			cc := a.E.MtlsClientCert
			if util.IsStringEmpty(cc.ClientCert) || util.IsStringEmpty(cc.ClientKey) {
				return nil, &ServiceError{ErrMsg: "mtls_client_cert requires both client_cert and client_key"}
			}

			// Validate the certificate and key pair (checks expiration and matching)
			_, err := config.LoadClientCertificate(cc.ClientCert, cc.ClientKey)
			if err != nil {
				return nil, &ServiceError{ErrMsg: fmt.Sprintf("invalid mTLS client certificate: %v", err)}
			}

			endpoint.MtlsClientCert = a.E.MtlsClientCert.Transform()
		}
	}

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

func (a *CreateEndpointService) ValidateEndpoint(ctx context.Context, enforceSecure bool, mtlsClientCert *models.MtlsClientCert) (string, error) {
	if util.IsStringEmpty(a.E.URL) {
		return "", ErrEndpointURLRequired
	}

	u, pingErr := url.Parse(a.E.URL)
	if pingErr != nil {
		return "", pingErr
	}

	switch u.Scheme {
	case "http":
		if enforceSecure {
			return "", ErrHTTPSOnly
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

		// Load mTLS client certificate if provided for ping validation
		// Note: This is best-effort. If cert loading fails here, it will be properly
		// validated later in the Run() method before persisting to database.
		var mtlsCert *tls.Certificate
		if mtlsClientCert != nil && !util.IsStringEmpty(mtlsClientCert.ClientCert) && !util.IsStringEmpty(mtlsClientCert.ClientKey) {
			cert, certErr := config.LoadClientCertificate(mtlsClientCert.ClientCert, mtlsClientCert.ClientKey)
			if certErr != nil {
				// Log warning but don't fail - validation will happen later
				log.FromContext(ctx).WithError(certErr).Warn("failed to load mTLS cert for ping, will validate later")
			} else {
				mtlsCert = cert
			}
		}

		pingErr = dispatcher.Ping(ctx, a.E.URL, 10*time.Second, a.E.ContentType, mtlsCert)
		if pingErr != nil {
			if cfg.Dispatcher.SkipPingValidation {
				log.FromContext(ctx).Warnf("failed to ping tls endpoint (validation skipped): %v", pingErr)
			} else {
				log.FromContext(ctx).Errorf("failed to ping tls endpoint: %v", pingErr)
				return "", fmt.Errorf("endpoint validation failed: %w", pingErr)
			}
		}
	default:
		return "", ErrInvalidEndpointScheme
	}

	return u.String(), nil
}

func ValidateEndpointAuthentication(auth *datastore.EndpointAuthentication) (*datastore.EndpointAuthentication, error) {
	if auth != nil && !util.IsStringEmpty(string(auth.Type)) {
		if err := util.Validate(auth); err != nil {
			return nil, err
		}

		if auth.Type == datastore.APIKeyAuthentication {
			if auth.ApiKey == nil || util.IsStringEmpty(auth.ApiKey.HeaderValue) {
				return nil, util.NewServiceError(http.StatusBadRequest, ErrAPIKeyFieldRequired)
			}
		}

		return auth, nil
	}

	return nil, nil
}
