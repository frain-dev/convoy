package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/net"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

// createOAuth2TokenGetter creates an OAuth2TokenGetter for ping validation.
// It uses a noop cache since this is a one-time validation.
func createOAuth2TokenGetter(auth *models.EndpointAuthentication, endpointURL string, existingEndpointID string, logger log.StdLogger) net.OAuth2TokenGetter {
	if auth == nil || auth.Type != datastore.OAuth2Authentication || auth.OAuth2 == nil {
		return nil
	}

	return createOAuth2TokenGetterFromDatastore(auth.OAuth2.Transform(), endpointURL, existingEndpointID, logger)
}

// createOAuth2TokenGetterFromDatastore creates an OAuth2TokenGetter from a datastore OAuth2 config.
func createOAuth2TokenGetterFromDatastore(oauth2 *datastore.OAuth2, endpointURL string, existingEndpointID string, logger log.StdLogger) net.OAuth2TokenGetter {
	if oauth2 == nil {
		return nil
	}

	tempEndpointID := existingEndpointID
	if tempEndpointID == "" {
		tempEndpointID = ulid.Make().String()
	}

	tempEndpoint := &datastore.Endpoint{
		UID: tempEndpointID,
		Url: endpointURL,
		Authentication: &datastore.EndpointAuthentication{
			Type:   datastore.OAuth2Authentication,
			OAuth2: oauth2,
		},
	}

	noopCache := ncache.NewNoopCache()
	oauth2TokenService := NewOAuth2TokenService(noopCache, logger)

	return func(ctx context.Context) (string, error) {
		return oauth2TokenService.GetAuthorizationHeader(ctx, tempEndpoint)
	}
}

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

	// Check license before allowing OAuth2 configuration
	if auth != nil && auth.Type == datastore.OAuth2Authentication {
		if !a.Licenser.OAuth2EndpointAuth() {
			return nil, &ServiceError{ErrMsg: ErrOAuth2FeatureUnavailable}
		}
	}

	endpoint.Authentication = auth

	// Set mTLS client certificate if provided
	if a.E.MtlsClientCert != nil {
		// Check license before allowing mTLS configuration
		if !a.Licenser.MutualTLS() {
			return nil, &ServiceError{ErrMsg: ErrMutualTLSFeatureUnavailable}
		}

		// Validate both fields provided together
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

		oauth2TokenGetter := createOAuth2TokenGetter(a.E.Authentication, a.E.URL, "", a.Logger)

		pingErr = dispatcher.Ping(ctx, a.E.URL, 10*time.Second, a.E.ContentType, mtlsCert, oauth2TokenGetter)
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

func ValidateEndpointAuthentication(auth *datastore.EndpointAuthentication) (*datastore.EndpointAuthentication, error) {
	if auth != nil && !util.IsStringEmpty(string(auth.Type)) {
		if err := util.Validate(auth); err != nil {
			return nil, err
		}

		switch auth.Type {
		case datastore.APIKeyAuthentication:
			if auth.ApiKey == nil {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("api key field is required"))
			}
			if util.IsStringEmpty(auth.ApiKey.HeaderName) || util.IsStringEmpty(auth.ApiKey.HeaderValue) {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("api key header_name and header_value are required"))
			}

		case datastore.OAuth2Authentication:
			if auth.OAuth2 == nil {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 configuration is required"))
			}
			if util.IsStringEmpty(auth.OAuth2.URL) {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 url is required"))
			}
			if util.IsStringEmpty(auth.OAuth2.ClientID) {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 client_id is required"))
			}
			if util.IsStringEmpty(string(auth.OAuth2.AuthenticationType)) {
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 authentication_type is required"))
			}

			// Validate authentication type specific fields
			switch auth.OAuth2.AuthenticationType {
			case datastore.SharedSecretAuth:
				if util.IsStringEmpty(auth.OAuth2.ClientSecret) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 client_secret is required for shared_secret authentication"))
				}
			case datastore.ClientAssertionAuth:
				if auth.OAuth2.SigningKey == nil {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key is required for client_assertion authentication"))
				}
				// Validate JWK fields
				if util.IsStringEmpty(auth.OAuth2.SigningKey.Kty) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.kty is required"))
				}
				if util.IsStringEmpty(auth.OAuth2.SigningKey.Kid) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.kid is required"))
				}
				if util.IsStringEmpty(auth.OAuth2.SigningKey.D) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.d (private key) is required"))
				}
				// Validate ES256 algorithm requirements
				if auth.OAuth2.SigningAlgorithm == "ES256" || auth.OAuth2.SigningAlgorithm == "" {
					if auth.OAuth2.SigningKey.Kty != "EC" {
						return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.kty must be 'EC' for ES256 algorithm"))
					}
					if auth.OAuth2.SigningKey.Crv != "P-256" {
						return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.crv must be 'P-256' for ES256 algorithm"))
					}
					if util.IsStringEmpty(auth.OAuth2.SigningKey.X) || util.IsStringEmpty(auth.OAuth2.SigningKey.Y) {
						return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 signing_key.x and signing_key.y are required for EC keys"))
					}
				}
				if util.IsStringEmpty(auth.OAuth2.Issuer) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 issuer is required for client_assertion authentication"))
				}
				if util.IsStringEmpty(auth.OAuth2.Subject) {
					return nil, util.NewServiceError(http.StatusBadRequest, errors.New("oauth2 subject is required for client_assertion authentication"))
				}
			default:
				return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("unsupported oauth2 authentication_type: %s", auth.OAuth2.AuthenticationType))
			}

			// Validate token URL format
			_, err := url.Parse(auth.OAuth2.URL)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("invalid oauth2 url: %w", err))
			}
		}

		return auth, nil
	}

	return nil, nil
}
