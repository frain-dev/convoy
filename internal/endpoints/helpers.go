package endpoints

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/endpoints/repo"
	"github.com/frain-dev/convoy/pkg/constants"
)

// ============================================================================
// Intermediate struct for row-to-endpoint conversion
// ============================================================================

// endpointFields is an intermediate struct that normalises all sqlc-generated
// row types (which share identical field layouts) into a single representation
// so that the actual conversion logic only needs to be written once.
type endpointFields struct {
	ID                                  string
	Name                                string
	Status                              string
	OwnerID                             pgtype.Text
	Url                                 string
	Description                         pgtype.Text
	HttpTimeout                         int32
	RateLimit                           int32
	RateLimitDuration                   int32
	AdvancedSignatures                  bool
	SlackWebhookUrl                     pgtype.Text
	SupportEmail                        pgtype.Text
	AppID                               pgtype.Text
	ProjectID                           string
	Secrets                             []byte
	CreatedAt                           pgtype.Timestamptz
	UpdatedAt                           pgtype.Timestamptz
	AuthenticationType                  pgtype.Text
	AuthenticationTypeApiKeyHeaderName  pgtype.Text
	AuthenticationTypeApiKeyHeaderValue pgtype.Text
	MtlsClientCert                      []byte
	Oauth2Config                        []byte
	BasicAuthConfig                     []byte
	ContentType                         string
}

// ============================================================================
// Row → Endpoint conversion
// ============================================================================

// rowToEndpoint converts any of the sqlc-generated row types into a
// *datastore.Endpoint. It uses a type switch to populate the common
// endpointFields struct, then delegates to fieldsToEndpoint.
func rowToEndpoint(row any) (*datastore.Endpoint, error) {
	var f endpointFields

	switch r := row.(type) {
	case repo.FindEndpointByIDRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FindEndpointsByIDsRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FindEndpointsByAppIDRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FindEndpointsByOwnerIDRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FindEndpointByTargetURLRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FetchEndpointsPagedForwardRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.FetchEndpointsPagedBackwardRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.UpdateEndpointStatusRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	case repo.UpdateEndpointSecretsRow:
		f = endpointFields{
			ID: r.ID, Name: r.Name, Status: r.Status, OwnerID: r.OwnerID,
			Url: r.Url, Description: r.Description, HttpTimeout: r.HttpTimeout,
			RateLimit: r.RateLimit, RateLimitDuration: r.RateLimitDuration,
			AdvancedSignatures: r.AdvancedSignatures, SlackWebhookUrl: r.SlackWebhookUrl,
			SupportEmail: r.SupportEmail, AppID: r.AppID, ProjectID: r.ProjectID,
			Secrets: r.Secrets, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AuthenticationType:                  r.AuthenticationType,
			AuthenticationTypeApiKeyHeaderName:  r.AuthenticationTypeApiKeyHeaderName,
			AuthenticationTypeApiKeyHeaderValue: r.AuthenticationTypeApiKeyHeaderValue,
			MtlsClientCert:                      r.MtlsClientCert, Oauth2Config: r.Oauth2Config,
			BasicAuthConfig: r.BasicAuthConfig, ContentType: r.ContentType,
		}
	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	return fieldsToEndpoint(f)
}

// fieldsToEndpoint converts the normalised endpointFields into a
// *datastore.Endpoint, handling JSON unmarshalling and type conversions.
func fieldsToEndpoint(f endpointFields) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{
		UID:                f.ID,
		Name:               f.Name,
		Status:             datastore.EndpointStatus(f.Status),
		OwnerID:            common.PgTextToString(f.OwnerID),
		Url:                f.Url,
		Description:        common.PgTextToString(f.Description),
		SlackWebhookURL:    common.PgTextToString(f.SlackWebhookUrl),
		SupportEmail:       common.PgTextToString(f.SupportEmail),
		AppID:              common.PgTextToString(f.AppID),
		ProjectID:          f.ProjectID,
		HttpTimeout:        uint64(f.HttpTimeout),
		RateLimit:          int(f.RateLimit),
		RateLimitDuration:  uint64(f.RateLimitDuration),
		AdvancedSignatures: f.AdvancedSignatures,
		ContentType:        f.ContentType,
		CreatedAt:          common.PgTimestamptzToTime(f.CreatedAt),
		UpdatedAt:          common.PgTimestamptzToTime(f.UpdatedAt),
	}

	// Unmarshal secrets
	if len(f.Secrets) > 0 {
		var secrets datastore.Secrets
		if err := json.Unmarshal(f.Secrets, &secrets); err != nil {
			return nil, fmt.Errorf("failed to unmarshal secrets: %w", err)
		}
		endpoint.Secrets = secrets
	}

	// Unmarshal mTLS client certificate
	if len(f.MtlsClientCert) > 0 {
		var mtls datastore.MtlsClientCert
		if err := json.Unmarshal(f.MtlsClientCert, &mtls); err != nil {
			return nil, fmt.Errorf("failed to unmarshal mtls_client_cert: %w", err)
		}
		endpoint.MtlsClientCert = &mtls
	}

	// Build authentication
	auth := &datastore.EndpointAuthentication{
		Type: datastore.EndpointAuthenticationType(common.PgTextToString(f.AuthenticationType)),
	}

	headerName := common.PgTextToString(f.AuthenticationTypeApiKeyHeaderName)
	headerValue := common.PgTextToString(f.AuthenticationTypeApiKeyHeaderValue)
	if headerName != "" || headerValue != "" {
		auth.ApiKey = &datastore.ApiKey{
			HeaderName:  headerName,
			HeaderValue: headerValue,
		}
	}

	// Unmarshal OAuth2 config
	if len(f.Oauth2Config) > 0 {
		var oauth2 datastore.OAuth2
		if err := json.Unmarshal(f.Oauth2Config, &oauth2); err != nil {
			return nil, fmt.Errorf("failed to unmarshal oauth2_config: %w", err)
		}
		auth.OAuth2 = &oauth2
		auth.Type = datastore.OAuth2Authentication
	}

	// Unmarshal BasicAuth config
	if len(f.BasicAuthConfig) > 0 {
		var basicAuth datastore.BasicAuth
		if err := json.Unmarshal(f.BasicAuthConfig, &basicAuth); err != nil {
			return nil, fmt.Errorf("failed to unmarshal basic_auth_config: %w", err)
		}
		auth.BasicAuth = &basicAuth
		auth.Type = datastore.BasicAuthentication
	}

	endpoint.Authentication = auth

	return endpoint, nil
}

// ============================================================================
// Content type validation
// ============================================================================

// validateAndSetContentType validates the given content type string and returns
// a normalised value. If empty, it defaults to application/json.
func validateAndSetContentType(contentType string) (string, error) {
	if contentType == "" {
		return constants.ContentTypeJSON, nil
	}

	if !constants.IsValidContentType(contentType) {
		return "", fmt.Errorf("invalid content type: %s", contentType)
	}

	return contentType, nil
}

// ============================================================================
// Endpoint → database parameter helpers
// ============================================================================

// marshalAuthFields extracts authentication configuration from an endpoint and
// returns the individual database column values.
func marshalAuthFields(endpoint *datastore.Endpoint) (apiKeyHeaderName, apiKeyHeaderValue string, oauth2Config, basicAuthConfig []byte) {
	if endpoint.Authentication == nil {
		return "", "", nil, nil
	}

	auth := endpoint.Authentication

	switch auth.Type {
	case datastore.APIKeyAuthentication:
		if auth.ApiKey != nil {
			apiKeyHeaderName = auth.ApiKey.HeaderName
			apiKeyHeaderValue = auth.ApiKey.HeaderValue
		}
	case datastore.OAuth2Authentication:
		if auth.OAuth2 != nil {
			data, err := json.Marshal(auth.OAuth2)
			if err == nil {
				oauth2Config = data
			}
		}
	case datastore.BasicAuthentication:
		if auth.BasicAuth != nil {
			data, err := json.Marshal(auth.BasicAuth)
			if err == nil {
				basicAuthConfig = data
			}
		}
	}

	return apiKeyHeaderName, apiKeyHeaderValue, oauth2Config, basicAuthConfig
}

// secretsToJSON marshals the given secrets to JSON bytes.
// Returns nil if marshalling fails or if secrets is empty.
func secretsToJSON(secrets datastore.Secrets) []byte {
	if len(secrets) == 0 {
		return nil
	}

	data, err := json.Marshal(secrets)
	if err != nil {
		return nil
	}

	return data
}
