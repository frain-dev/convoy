package models

import (
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type CreateEndpoint struct {
	URL                string `json:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string `json:"secret"`
	OwnerID            string `json:"owner_id"`
	Description        string `json:"description"`
	AdvancedSignatures bool   `json:"advanced_signatures"`
	Name               string `json:"name" valid:"required~please provide your endpointName"`
	SupportEmail       string `json:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         bool   `json:"is_disabled"`
	SlackWebhookURL    string `json:"slack_webhook_url"`

	HttpTimeout       string                  `json:"http_timeout"`
	RateLimit         int                     `json:"rate_limit"`
	RateLimitDuration string                  `json:"rate_limit_duration"`
	Authentication    *EndpointAuthentication `json:"authentication"`
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

	HttpTimeout       string                  `json:"http_timeout"`
	RateLimit         int                     `json:"rate_limit"`
	RateLimitDuration string                  `json:"rate_limit_duration"`
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

	HttpTimeout       string                  `json:"http_timeout"`
	RateLimit         int                     `json:"rate_limit"`
	RateLimitDuration string                  `json:"rate_limit_duration"`
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
