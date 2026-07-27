package task

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/httpheader"
	log "github.com/frain-dev/convoy/pkg/logger"
)

var errEndpointAuthUnavailable = errors.New("endpoint authentication configured but could not be applied")

type endpointAuthDeps struct {
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	OAuth2TokenService         OAuth2TokenService
	OrganisationID             string
	Logger                     log.Logger
}

// resolveEndpointDeliveryHeaders applies endpoint auth to outbound delivery headers.
// Failure policy: fail closed when auth is configured but cannot be applied.
func resolveEndpointDeliveryHeaders(
	ctx context.Context,
	endpoint *datastore.Endpoint,
	eventHeaders httpheader.HTTPHeader,
	deps endpointAuthDeps,
) (httpheader.HTTPHeader, error) {
	if endpoint == nil || endpoint.Authentication == nil {
		return eventHeaders, nil
	}

	switch endpoint.Authentication.Type {
	case datastore.APIKeyAuthentication:
		if endpoint.Authentication.ApiKey == nil {
			return nil, fmt.Errorf("%w: api key config is nil", errEndpointAuthUnavailable)
		}
		headers := make(httpheader.HTTPHeader)
		headers[endpoint.Authentication.ApiKey.HeaderName] = []string{endpoint.Authentication.ApiKey.HeaderValue}
		headers.MergeHeaders(eventHeaders)
		return headers, nil
	case datastore.OAuth2Authentication:
		oauth2Enabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.OAuthTokenExchange, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, deps.OrganisationID)
		if !oauth2Enabled {
			return nil, fmt.Errorf("%w: oauth2 feature disabled", errEndpointAuthUnavailable)
		}
		if deps.OAuth2TokenService == nil {
			return nil, fmt.Errorf("%w: oauth2 token service unavailable", errEndpointAuthUnavailable)
		}
		authHeader, err := deps.OAuth2TokenService.GetAuthorizationHeader(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errEndpointAuthUnavailable, err)
		}
		headers := make(httpheader.HTTPHeader)
		headers["Authorization"] = []string{authHeader}
		headers.MergeHeaders(eventHeaders)
		return headers, nil
	case datastore.BasicAuthentication:
		basicAuthEnabled := deps.FeatureFlag.CanAccessOrgFeature(ctx, fflag.BasicAuthEndpoint, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, deps.OrganisationID)
		if !basicAuthEnabled {
			return nil, fmt.Errorf("%w: basic auth feature disabled", errEndpointAuthUnavailable)
		}
		if endpoint.Authentication.BasicAuth == nil {
			return nil, fmt.Errorf("%w: basic auth config is nil", errEndpointAuthUnavailable)
		}
		if endpoint.Authentication.BasicAuth.UserName == "" && endpoint.Authentication.BasicAuth.Password == "" {
			return nil, fmt.Errorf("%w: basic auth credentials are empty", errEndpointAuthUnavailable)
		}
		headers := make(httpheader.HTTPHeader)
		credentials := base64.StdEncoding.EncodeToString(
			[]byte(endpoint.Authentication.BasicAuth.UserName + ":" + endpoint.Authentication.BasicAuth.Password),
		)
		headers["Authorization"] = []string{"Basic " + credentials}
		headers.MergeHeaders(eventHeaders)
		return headers, nil
	default:
		return eventHeaders, nil
	}
}
