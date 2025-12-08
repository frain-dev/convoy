package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type UpdateEndpointService struct {
	Cache                      cache.Cache
	EndpointRepo               datastore.EndpointRepository
	ProjectRepo                datastore.ProjectRepository
	Licenser                   license.Licenser
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	DB                         database.Database
	Logger                     log.StdLogger
	E                          models.UpdateEndpoint
	Endpoint                   *datastore.Endpoint
	Project                    *datastore.Project
}

func (a *UpdateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	// Fetch the current endpoint from database first to get decrypted mTLS cert
	endpoint, err := a.EndpointRepo.FindEndpointByID(ctx, a.Endpoint.UID, a.Project.UID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	// Validate the endpoint URL with the existing endpoint data
	endpointUrl, err := a.ValidateEndpoint(ctx, a.Project.Config.SSL.EnforceSecureEndpoints, a.E.MtlsClientCert, endpoint)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	a.E.URL = endpointUrl

	endpoint, err = a.updateEndpoint(ctx, endpoint, a.E, a.Project)
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

func (a *UpdateEndpointService) ValidateEndpoint(ctx context.Context, enforceSecure bool, mtlsClientCert *models.MtlsClientCert, existingEndpoint *datastore.Endpoint) (string, error) {
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

		var mtlsCert *tls.Certificate

		if mtlsClientCert != nil && !util.IsStringEmpty(mtlsClientCert.ClientCert) && !util.IsStringEmpty(mtlsClientCert.ClientKey) {
			cert, certErr := config.LoadClientCertificate(mtlsClientCert.ClientCert, mtlsClientCert.ClientKey)
			if certErr != nil {
				log.FromContext(ctx).WithError(certErr).Warn("failed to load new mTLS cert for ping, will validate later")
			} else {
				mtlsCert = cert
			}
		} else if mtlsClientCert == nil && existingEndpoint != nil && existingEndpoint.MtlsClientCert != nil {
			cert, certErr := config.LoadClientCertificate(existingEndpoint.MtlsClientCert.ClientCert, existingEndpoint.MtlsClientCert.ClientKey)
			if certErr != nil {
				log.FromContext(ctx).WithError(certErr).Warn("failed to load existing mTLS cert for ping")
			} else {
				mtlsCert = cert
			}
		}

		contentType := ""
		if a.E.ContentType != nil {
			contentType = *a.E.ContentType
		}

		var oauth2TokenGetter net.OAuth2TokenGetter
		if a.E.Authentication != nil && a.E.Authentication.Type == datastore.OAuth2Authentication {
			// OAuth2 is being set or updated
			oauth2TokenGetter = createOAuth2TokenGetter(a.E.Authentication, a.E.URL, existingEndpoint.UID, a.Logger)
		} else if existingEndpoint != nil && existingEndpoint.Authentication != nil && existingEndpoint.Authentication.Type == datastore.OAuth2Authentication {
			// OAuth2 is not being updated, but endpoint already has OAuth2 - use existing config
			oauth2TokenGetter = createOAuth2TokenGetterFromDatastore(existingEndpoint.Authentication.OAuth2, a.E.URL, existingEndpoint.UID, a.Logger)
		}

		pingErr = dispatcher.Ping(ctx, net.PingOptions{
			Endpoint:          a.E.URL,
			Timeout:           10 * time.Second,
			ContentType:       contentType,
			MtlsCert:          mtlsCert,
			OAuth2TokenGetter: oauth2TokenGetter,
		})
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

//nolint:cyclop // Large function with many conditional branches for endpoint updates
func (a *UpdateEndpointService) updateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, e models.UpdateEndpoint, project *datastore.Project) (*datastore.Endpoint, error) {
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

	if e.ContentType != nil {
		endpoint.ContentType = *e.ContentType
	}

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

	// Check license before allowing OAuth2 configuration
	if auth != nil && auth.Type == datastore.OAuth2Authentication {
		if !a.Licenser.OAuth2EndpointAuth() {
			return nil, &ServiceError{ErrMsg: ErrOAuth2FeatureUnavailable}
		}

		// Check feature flag for OAuth2 using project's organisation ID
		oauth2Enabled := a.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, a.FeatureFlagFetcher, a.EarlyAdopterFeatureFetcher, a.Project.OrganisationID)
		if !oauth2Enabled {
			log.FromContext(ctx).Warn("OAuth2 configuration provided but feature flag not enabled, ignoring OAuth2 config")
			// Remove OAuth2 authentication if feature flag is disabled
			auth = nil
		}
	}

	endpoint.Authentication = auth

	// Update mTLS client certificate if provided
	if e.MtlsClientCert != nil {
		cc := e.MtlsClientCert

		if util.IsStringEmpty(cc.ClientCert) && util.IsStringEmpty(cc.ClientKey) {
			// Both empty means remove mTLS configuration
			endpoint.MtlsClientCert = nil
			// Clear cached certificate since it's being removed
			config.GetCertCache().Delete(endpoint.UID)
		} else {
			// Check license before allowing mTLS configuration
			if !a.Licenser.MutualTLS() {
				return nil, &ServiceError{ErrMsg: ErrMutualTLSFeatureUnavailable}
			}

			// Updating or setting new mTLS cert - both fields required
			mtlsEnabled := a.FeatureFlag.CanAccessOrgFeature(ctx, fflag.MTLS, a.FeatureFlagFetcher, a.EarlyAdopterFeatureFetcher, a.Project.OrganisationID)
			if !mtlsEnabled {
				log.FromContext(ctx).Warn("mTLS configuration provided but feature flag not enabled, ignoring mTLS config")
				endpoint.MtlsClientCert = nil
				config.GetCertCache().Delete(endpoint.UID)
			} else {
				if util.IsStringEmpty(cc.ClientCert) || util.IsStringEmpty(cc.ClientKey) {
					return nil, &ServiceError{ErrMsg: "mtls_client_cert requires both client_cert and client_key"}
				}

				// Validate the certificate and key pair (checks expiration and matching)
				_, err := config.LoadClientCertificate(cc.ClientCert, cc.ClientKey)
				if err != nil {
					return nil, &ServiceError{ErrMsg: fmt.Sprintf("invalid mTLS client certificate: %v", err)}
				}

				endpoint.MtlsClientCert = e.MtlsClientCert.Transform()

				// Clear cached certificate since it's being updated
				config.GetCertCache().Delete(endpoint.UID)
			}
		}
	}

	endpoint.UpdatedAt = time.Now()

	return endpoint, nil
}
