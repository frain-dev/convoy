package models

import (
	"net/http"
	"strings"

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

	// Convoy supports two [signature formats](https://getconvoy.io/docs/manual/signatures)
	// -- simple or advanced. If left unspecified, we default to false.
	AdvancedSignatures *bool `json:"advanced_signatures"`

	// Endpoint name.
	Name string `json:"name" valid:"required~please provide your endpointName"`

	// Endpoint developers support email. This is used for communicating endpoint state
	// changes. You should always turn this on when disabling endpoints are enabled.
	SupportEmail string `json:"support_email" valid:"email~please provide a valid email"`

	// This is used to manually enable/disable the endpoint.
	IsDisabled bool `json:"is_disabled"`

	// Slack webhook URL is an alternative method to support email where endpoint developers
	// can receive failure notifications.
	SlackWebhookURL string `json:"slack_webhook_url"`

	// Define endpoint http timeout in seconds.
	HttpTimeout int64 `json:"http_timeout" copier:"-"`

	// Rate limit is the total number of requests to be sent to an endpoint in
	// the time duration specified in RateLimitDuration
	RateLimit int `json:"rate_limit"`

	// Rate limit duration specifies the time range for the rate limit.
	RateLimitDuration int64 `json:"rate_limit_duration" copier:"-"`

	// This is used to define any custom authentication required by the endpoint. This
	// shouldn't be needed often because webhook endpoints usually should be exposed to
	// the internet.
	Authentication *EndpointAuthentication `json:"authentication"`

	// Deprecated but necessary for backward compatibility
	AppID string
}

func (cE *CreateEndpoint) Validate() error {
	return util.Validate(cE)
}

type UpdateEndpoint struct {
	URL                string  `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string  `json:"secret"`
	OwnerID            string  `json:"owner_id"`
	Description        string  `json:"description"`
	AdvancedSignatures *bool   `json:"advanced_signatures"`
	Name               *string `json:"name" valid:"required~please provide your endpointName"`
	SupportEmail       *string `json:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         *bool   `json:"is_disabled"`
	SlackWebhookURL    *string `json:"slack_webhook_url"`

	HttpTimeout       int64                   `json:"http_timeout"`
	RateLimit         int                     `json:"rate_limit"`
	RateLimitDuration int64                   `json:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication"`
}

func (uE *UpdateEndpoint) Validate() error {
	return util.Validate(uE)
}

type DynamicEndpoint struct {
	URL                string `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string `json:"secret"`
	OwnerID            string `json:"owner_id"`
	Description        string `json:"description"`
	AdvancedSignatures bool   `json:"advanced_signatures"`
	Name               string `json:"name"`
	SupportEmail       string `json:"support_email"`
	IsDisabled         bool   `json:"is_disabled"`
	SlackWebhookURL    string `json:"slack_webhook_url"`

	HttpTimeout       int64                   `json:"http_timeout"`
	RateLimit         int                     `json:"rate_limit"`
	RateLimitDuration int64                   `json:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication"`
	// Deprecated but necessary for backward compatibility
	AppID string
}

func (dE *DynamicEndpoint) Validate() error {
	return util.Validate(dE)
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
	Type   datastore.EndpointAuthenticationType `json:"type,omitempty" valid:"optional,in(api_key)~unsupported authentication type"`
	ApiKey *ApiKey                              `json:"api_key"`
}

func (ea *EndpointAuthentication) Transform() *datastore.EndpointAuthentication {
	if ea == nil {
		return nil
	}

	return &datastore.EndpointAuthentication{
		Type:   ea.Type,
		ApiKey: ea.ApiKey.transform(),
	}
}

type EndpointResponse struct {
	*datastore.Endpoint
}
