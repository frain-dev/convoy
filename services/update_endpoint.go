package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/net"

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

func (a *UpdateEndpointService) ValidateEndpoint(ctx context.Context, enforceSecure bool, mtlsClientCert *models.MtlsClientCert, existingEndpoint *datastore.Endpoint) (string, error) {
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

		// Load mTLS client certificate if provided for ping validation
		// If user is not updating cert ([REDACTED]), use existing cert from database
		var mtlsCert *tls.Certificate
		if mtlsClientCert != nil && mtlsClientCert.ClientKey == "[REDACTED]" {
			// User is not updating cert - use existing cert from database for ping
			if existingEndpoint != nil && existingEndpoint.MtlsClientCert != nil {
				cert, certErr := config.LoadClientCertificate(existingEndpoint.MtlsClientCert.ClientCert, existingEndpoint.MtlsClientCert.ClientKey)
				if certErr != nil {
					log.FromContext(ctx).WithError(certErr).Warn("failed to load existing mTLS cert for ping")
				} else {
					mtlsCert = cert
				}
			}
		} else if mtlsClientCert != nil && !util.IsStringEmpty(mtlsClientCert.ClientCert) && !util.IsStringEmpty(mtlsClientCert.ClientKey) {
			// User is updating cert - validate and use new cert for ping
			cert, certErr := config.LoadClientCertificate(mtlsClientCert.ClientCert, mtlsClientCert.ClientKey)
			if certErr != nil {
				// Log warning but don't fail - validation will happen later
				log.FromContext(ctx).WithError(certErr).Warn("failed to load new mTLS cert for ping, will validate later")
			} else {
				mtlsCert = cert
			}
		}

		contentType := ""
		if a.E.ContentType != nil {
			contentType = *a.E.ContentType
		}
		pingErr = dispatcher.Ping(ctx, a.E.URL, 10*time.Second, contentType, mtlsCert)
		if pingErr != nil {
			if cfg.Dispatcher.SkipPingValidation {
				log.FromContext(ctx).Warnf("failed to ping tls endpoint (validation skipped): %v", pingErr)
			} else {
				log.FromContext(ctx).Errorf("failed to ping tls endpoint: %v", pingErr)
				return "", fmt.Errorf("endpoint validation failed: %w", pingErr)
			}
		}
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

	endpoint.Authentication = auth

	// Update mTLS client certificate if provided
	if e.MtlsClientCert != nil {
		cc := e.MtlsClientCert

		// Skip update if client_key is redacted placeholder (user is not updating mTLS cert)
		if cc.ClientKey == "[REDACTED]" {
			// Keep existing mTLS cert unchanged
		} else if util.IsStringEmpty(cc.ClientCert) && util.IsStringEmpty(cc.ClientKey) {
			// Both empty means remove mTLS configuration
			endpoint.MtlsClientCert = nil
			// Clear cached certificate since it's being removed
			config.GetCertCache().Delete(endpoint.UID)
		} else {
			// Updating or setting new mTLS cert - both fields required
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

	endpoint.UpdatedAt = time.Now()

	return endpoint, nil
}
