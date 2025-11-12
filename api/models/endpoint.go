package models

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type CreateEndpoint struct {
	// URL is the endpoint's URL prefixed with https. non-https urls are currently
	// not supported.
	URL string `json:"url" valid:"required~please provide a url for your endpoint"`

	// Endpoint's webhook secret. If not provided, Convoy autogenerates one for the endpoint.
	Secret string `json:"secret"`

	// The OwnerID is used to group more than one endpoint together to achieve
	// [fanout](https://getconvoy.io/docs/manual/endpoints#Endpoint%20Owner%20ID)
	OwnerID string `json:"owner_id"`

	// Human-readable description of the endpoint. Think of this as metadata describing
	// the endpoint
	Description string `json:"description"`

	// Convoy supports two [signature formats](https://getconvoy.io/docs/product-manual/signatures)
	// -- simple or advanced. If left unspecified, we default to false.
	AdvancedSignatures *bool `json:"advanced_signatures"`

	// Endpoint name.
	Name string `json:"name" valid:"required~please provide your endpoint name"`

	// Endpoint developers support email. This is used for communicating endpoint state
	// changes. You should always turn this on when disabling endpoints are enabled.
	SupportEmail string `json:"support_email" valid:"email~please provide a valid email"`

	// This is used to manually enable/disable the endpoint.
	IsDisabled bool `json:"is_disabled"`

	// Slack webhook URL is an alternative method to support email where endpoint developers
	// can receive failure notifications on a slack channel.
	SlackWebhookURL string `json:"slack_webhook_url"`

	// Define endpoint http timeout in seconds.
	HttpTimeout uint64 `json:"http_timeout" copier:"-"`

	// Rate limit is the total number of requests to be sent to an endpoint in
	// the time duration specified in RateLimitDuration
	RateLimit int `json:"rate_limit"`

	// Rate limit duration specifies the time range for the rate limit.
	RateLimitDuration uint64 `json:"rate_limit_duration" copier:"-"`

	// Content type for the endpoint. Defaults to application/json if not specified.
	ContentType string `json:"content_type"`

	// This is used to define any custom authentication required by the endpoint. This
	// shouldn't be needed often because webhook endpoints usually should be exposed to
	// the internet.
	Authentication *EndpointAuthentication `json:"authentication"`

	// mTLS client certificate configuration for the endpoint
	MtlsClientCert *MtlsClientCert `json:"mtls_client_cert,omitempty"`

	// Deprecated but necessary for backward compatibility
	AppID string
}

func (cE *CreateEndpoint) Validate() error {
	return util.Validate(cE)
}

type UpdateEndpoint struct {
	// URL is the endpoint's URL prefixed with https. non-https urls are currently
	// not supported.
	URL string `json:"url" valid:"required~please provide a url for your endpoint"`

	// Endpoint's webhook secret. If not provided, Convoy autogenerates one for the endpoint.
	Secret string `json:"secret"`

	// The OwnerID is used to group more than one endpoint together to achieve
	// [fanout](https://getconvoy.io/docs/manual/endpoints#Endpoint%20Owner%20ID)
	OwnerID string `json:"owner_id"`

	// Human-readable description of the endpoint. Think of this as metadata describing
	// the endpoint
	Description string `json:"description"`

	// Convoy supports two [signature formats](https://getconvoy.io/docs/product-manual/signatures)
	// -- simple or advanced. If left unspecified, we default to false.
	AdvancedSignatures *bool `json:"advanced_signatures"`

	// Endpoint name.

	Name *string `json:"name" valid:"required~please provide your endpointName"`

	// Endpoint developers support email. This is used for communicating endpoint state
	// changes. You should always turn this on when disabling endpoints are enabled.
	SupportEmail *string `json:"support_email" valid:"email~please provide a valid email"`

	// This is used to manually enable/disable the endpoint.
	IsDisabled *bool `json:"is_disabled"`

	// Slack webhook URL is an alternative method to support email where endpoint developers
	// can receive failure notifications on a slack channel.
	SlackWebhookURL *string `json:"slack_webhook_url"`

	// Define endpoint http timeout in seconds.
	HttpTimeout uint64 `json:"http_timeout" copier:"-"`

	// Rate limit is the total number of requests to be sent to an endpoint in
	// the time duration specified in RateLimitDuration
	RateLimit int `json:"rate_limit"`

	// Rate limit duration specifies the time range for the rate limit.
	RateLimitDuration uint64 `json:"rate_limit_duration" copier:"-"`

	// Content type for the endpoint. Defaults to application/json if not specified.
	ContentType *string `json:"content_type"`

	// This is used to define any custom authentication required by the endpoint. This
	// shouldn't be needed often because webhook endpoints usually should be exposed to
	// the internet.
	Authentication *EndpointAuthentication `json:"authentication"`

	// mTLS client certificate configuration for the endpoint
	MtlsClientCert *MtlsClientCert `json:"mtls_client_cert,omitempty"`
}

func (uE *UpdateEndpoint) Validate() error {
	return util.Validate(uE)
}

type QueryListEndpoint struct {
	// The name of the endpoint
	Name string `json:"q" example:"endpoint-1"`
	// The owner ID of the endpoint
	OwnerID string `json:"ownerId" example:"01H0JA5MEES38RRK3HTEJC647K"`
	Pageable
}

type QueryListEndpointResponse struct {
	datastore.Pageable
	*datastore.Filter
}

func (q *QueryListEndpoint) Transform(r *http.Request) *QueryListEndpointResponse {
	return &QueryListEndpointResponse{
		Pageable: m.GetPageableFromContext(r.Context()),
		Filter: &datastore.Filter{
			Query:   strings.TrimSpace(r.URL.Query().Get("q")),
			OwnerID: r.URL.Query().Get("ownerId"),
		},
	}
}

type EndpointAuthentication struct {
	Type   datastore.EndpointAuthenticationType `json:"type,omitempty" valid:"optional,in(api_key|oauth2)~unsupported authentication type"`
	ApiKey *ApiKey                              `json:"api_key,omitempty"`
	OAuth2 *OAuth2                              `json:"oauth2,omitempty"`
}

// OAuth2SigningKey represents a JWK-formatted signing key for client assertion
type OAuth2SigningKey struct {
	Kty string `json:"kty"` // Key type: "EC" or "RSA"

	// EC (Elliptic Curve) key fields
	Crv string `json:"crv,omitempty"` // Curve: "P-256", "P-384", "P-521"
	X   string `json:"x,omitempty"`   // X coordinate (EC only)
	Y   string `json:"y,omitempty"`   // Y coordinate (EC only)
	D   string `json:"d,omitempty"`   // Private key (EC) or private exponent (RSA)

	// RSA key fields
	N  string `json:"n,omitempty"`  // RSA modulus (RSA only)
	E  string `json:"e,omitempty"`  // RSA public exponent (RSA only)
	P  string `json:"p,omitempty"`  // RSA first prime factor (RSA private key only)
	Q  string `json:"q,omitempty"`  // RSA second prime factor (RSA private key only)
	Dp string `json:"dp,omitempty"` // RSA first factor CRT exponent (RSA private key only)
	Dq string `json:"dq,omitempty"` // RSA second factor CRT exponent (RSA private key only)
	Qi string `json:"qi,omitempty"` // RSA first CRT coefficient (RSA private key only)

	Kid string `json:"kid"` // Key ID
}

// OAuth2FieldMapping allows custom field name mappings for token response
type OAuth2FieldMapping struct {
	AccessToken string `json:"access_token,omitempty"` // Field name for access token (e.g., "accessToken", "access_token", "token")
	TokenType   string `json:"token_type,omitempty"`   // Field name for token type (e.g., "tokenType", "token_type")
	ExpiresIn   string `json:"expires_in,omitempty"`   // Field name for expiry time (e.g., "expiresIn", "expires_in", "expiresAt")
}

// OAuth2 holds OAuth2 authentication configuration
type OAuth2 struct {
	URL                string            `json:"url" valid:"required"`
	ClientID           string            `json:"client_id" valid:"required"`
	GrantType          string            `json:"grant_type,omitempty"`
	Scope              string            `json:"scope,omitempty"`
	Audience           string            `json:"audience,omitempty"`
	AuthenticationType string            `json:"authentication_type" valid:"required,in(shared_secret|client_assertion)~unsupported authentication type"`
	ClientSecret       string            `json:"client_secret,omitempty"`
	SigningKey         *OAuth2SigningKey `json:"signing_key,omitempty"`
	SigningAlgorithm   string            `json:"signing_algorithm,omitempty"`
	Issuer             string            `json:"issuer,omitempty"`
	Subject            string            `json:"subject,omitempty"`
	// Field mapping for flexible token response parsing
	FieldMapping *OAuth2FieldMapping `json:"field_mapping,omitempty"`
	// Expiry time unit (seconds, milliseconds, minutes, hours)
	ExpiryTimeUnit string `json:"expiry_time_unit,omitempty" valid:"optional,in(seconds|milliseconds|minutes|hours)~unsupported expiry time unit"`
}

// Transform converts the API OAuth2 model to the datastore model
func (o *OAuth2) Transform() *datastore.OAuth2 {
	if o == nil {
		return nil
	}

	var signingKey *datastore.OAuth2SigningKey
	if o.SigningKey != nil {
		signingKey = &datastore.OAuth2SigningKey{
			Kty: o.SigningKey.Kty,
			// EC fields
			Crv: o.SigningKey.Crv,
			X:   o.SigningKey.X,
			Y:   o.SigningKey.Y,
			// RSA fields
			N:  o.SigningKey.N,
			E:  o.SigningKey.E,
			P:  o.SigningKey.P,
			Q:  o.SigningKey.Q,
			Dp: o.SigningKey.Dp,
			Dq: o.SigningKey.Dq,
			Qi: o.SigningKey.Qi,
			// Common fields
			D:   o.SigningKey.D,
			Kid: o.SigningKey.Kid,
		}
	}

	var fieldMapping *datastore.OAuth2FieldMapping
	if o.FieldMapping != nil {
		fieldMapping = &datastore.OAuth2FieldMapping{
			AccessToken: o.FieldMapping.AccessToken,
			TokenType:   o.FieldMapping.TokenType,
			ExpiresIn:   o.FieldMapping.ExpiresIn,
		}
	}

	expiryTimeUnit := datastore.ExpiryTimeUnitSeconds // Default to seconds
	if o.ExpiryTimeUnit != "" {
		expiryTimeUnit = datastore.OAuth2ExpiryTimeUnit(o.ExpiryTimeUnit)
	}

	return &datastore.OAuth2{
		URL:                o.URL,
		ClientID:           o.ClientID,
		GrantType:          o.GrantType,
		Scope:              o.Scope,
		Audience:           o.Audience,
		AuthenticationType: datastore.OAuth2AuthenticationType(o.AuthenticationType),
		ClientSecret:       o.ClientSecret,
		SigningKey:         signingKey,
		SigningAlgorithm:   o.SigningAlgorithm,
		Issuer:             o.Issuer,
		Subject:            o.Subject,
		FieldMapping:       fieldMapping,
		ExpiryTimeUnit:     expiryTimeUnit,
	}
}

// MtlsClientCert holds the client certificate and key configuration for mTLS
type MtlsClientCert struct {
	// ClientCert is the client certificate PEM string
	ClientCert string `json:"client_cert,omitempty"`
	// ClientKey is the client private key PEM string
	ClientKey string `json:"client_key,omitempty"`
}

func (mc *MtlsClientCert) Transform() *datastore.MtlsClientCert {
	if mc == nil {
		return nil
	}

	return &datastore.MtlsClientCert{
		ClientCert: mc.ClientCert,
		ClientKey:  mc.ClientKey,
	}
}

func (ea *EndpointAuthentication) Transform() *datastore.EndpointAuthentication {
	if ea == nil {
		return nil
	}

	return &datastore.EndpointAuthentication{
		Type:   ea.Type,
		ApiKey: ea.ApiKey.transform(),
		OAuth2: ea.OAuth2.Transform(),
	}
}

type EndpointResponse struct {
	*datastore.Endpoint
}

// MarshalJSON redacts sensitive fields before serializing the endpoint response.
// Specifically, it removes the mTLS client private key from the JSON output.
func (er EndpointResponse) MarshalJSON() ([]byte, error) {
	if er.Endpoint == nil {
		return []byte("null"), nil
	}

	// Create a shallow copy to avoid mutating the original
	e := *er.Endpoint
	if e.MtlsClientCert != nil {
		mtls := *e.MtlsClientCert
		// Redact private key from API responses - show placeholder if key exists
		if mtls.ClientKey != "" {
			mtls.ClientKey = "[REDACTED]"
		}
		e.MtlsClientCert = &mtls
	}

	return json.Marshal(&e)
}

// TestOAuth2Request represents a request to test OAuth2 connection
type TestOAuth2Request struct {
	OAuth2 *OAuth2 `json:"oauth2" valid:"required"`
}

func (t *TestOAuth2Request) Validate() error {
	return util.Validate(t)
}

// TestOAuth2Response represents the response from OAuth2 connection test
type TestOAuth2Response struct {
	Success     bool      `json:"success"`
	AccessToken string    `json:"access_token,omitempty"`
	TokenType   string    `json:"token_type,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Error       string    `json:"error,omitempty"`
	Message     string    `json:"message,omitempty"`
}
