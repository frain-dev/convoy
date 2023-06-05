package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"gopkg.in/guregu/null.v4"
)

type Organisation struct {
	Name         string `json:"name" bson:"name"`
	CustomDomain string `json:"custom_domain" bson:"custom_domain"`
}

type OrganisationInvite struct {
	InviteeEmail string    `json:"invitee_email" valid:"required~please provide a valid invitee email,email"`
	Role         auth.Role `json:"role" bson:"role"`
}

type APIKey struct {
	Name      string            `json:"name"`
	Role      Role              `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt null.Time         `json:"expires_at"`
}

type PersonalAPIKey struct {
	Name       string `json:"name"`
	Expiration int    `json:"expiration"`
}

type Role struct {
	Type    auth.RoleType `json:"type"`
	Project string        `json:"project"`
	App     string        `json:"app,omitempty"`
}

type UpdateOrganisationMember struct {
	Role auth.Role `json:"role" bson:"role"`
}

type APIKeyByIDResponse struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Role      auth.Role         `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt null.Time         `json:"expires_at,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	UpdatedAt time.Time         `json:"updated_at,omitempty"`
}

type APIKeyResponse struct {
	APIKey
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type PortalAPIKeyResponse struct {
	Key        string    `json:"key"`
	Role       auth.Role `json:"role"`
	Url        string    `json:"url,omitempty"`
	Type       string    `json:"key_type"`
	EndpointID string    `json:"endpoint_id,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
}

type UserInviteTokenResponse struct {
	Token *datastore.OrganisationInvite `json:"token"`
	User  *datastore.User               `json:"user"`
}

type Endpoint struct {
	URL                string `json:"url" bson:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string `json:"secret" bson:"secret"`
	OwnerID            string `json:"owner_id" bson:"owner_id"`
	Description        string `json:"description" bson:"description"`
	AdvancedSignatures bool   `json:"advanced_signatures" bson:"advanced_signatures"`
	Name               string `json:"name" bson:"name" valid:"required~please provide your endpointName"`
	SupportEmail       string `json:"support_email" bson:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         bool   `json:"is_disabled"`
	SlackWebhookURL    string `json:"slack_webhook_url" bson:"slack_webhook_url"`

	HttpTimeout       string                            `json:"http_timeout" bson:"http_timeout"`
	RateLimit         int                               `json:"rate_limit" bson:"rate_limit"`
	RateLimitDuration string                            `json:"rate_limit_duration" bson:"rate_limit_duration"`
	Authentication    *datastore.EndpointAuthentication `json:"authentication"`
	AppID             string                            // Deprecated but necessary for backward compatibility
}

type UpdateEndpoint struct {
	URL                string  `json:"url" bson:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string  `json:"secret" bson:"secret"`
	OwnerID            string  `json:"owner_id" bson:"owner_id"`
	Description        string  `json:"description" bson:"description"`
	AdvancedSignatures *bool   `json:"advanced_signatures" bson:"advanced_signatures"`
	Name               *string `json:"name" bson:"name" valid:"required~please provide your endpointName"`
	SupportEmail       *string `json:"support_email" bson:"support_email" valid:"email~please provide a valid email"`
	IsDisabled         *bool   `json:"is_disabled"`
	SlackWebhookURL    *string `json:"slack_webhook_url" bson:"slack_webhook_url"`

	HttpTimeout       string                            `json:"http_timeout" bson:"http_timeout"`
	RateLimit         int                               `json:"rate_limit" bson:"rate_limit"`
	RateLimitDuration string                            `json:"rate_limit_duration" bson:"rate_limit_duration"`
	Authentication    *datastore.EndpointAuthentication `json:"authentication"`
}

type DynamicSubscription struct {
	Name            string                            `json:"name" bson:"name"`
	AlertConfig     *datastore.AlertConfiguration     `json:"alert_config,omitempty" bson:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration               `json:"retry_config,omitempty" bson:"retry_config,omitempty"`
	FilterConfig    *datastore.FilterConfiguration    `json:"filter_config,omitempty" bson:"filter_config,omitempty"`
	RateLimitConfig *datastore.RateLimitConfiguration `json:"rate_limit_config,omitempty" bson:"rate_limit_config,omitempty"`
}

type DynamicEvent struct {
	Endpoint     DynamicEndpoint     `json:"endpoint"`
	Subscription DynamicSubscription `json:"subscription"`
	Event        DynamicEventStub    `json:"event"`
}

type DynamicEndpoint struct {
	URL                string `json:"url" bson:"url" valid:"required~please provide a url for your endpoint"`
	Secret             string `json:"secret" bson:"secret"`
	OwnerID            string `json:"owner_id" bson:"owner_id"`
	Description        string `json:"description" bson:"description"`
	AdvancedSignatures bool   `json:"advanced_signatures" bson:"advanced_signatures"`
	Name               string `json:"name" bson:"name"`
	SupportEmail       string `json:"support_email" bson:"support_email"`
	IsDisabled         bool   `json:"is_disabled"`
	SlackWebhookURL    string `json:"slack_webhook_url" bson:"slack_webhook_url"`

	HttpTimeout       string                            `json:"http_timeout" bson:"http_timeout"`
	RateLimit         int                               `json:"rate_limit" bson:"rate_limit"`
	RateLimitDuration string                            `json:"rate_limit_duration" bson:"rate_limit_duration"`
	Authentication    *datastore.EndpointAuthentication `json:"authentication"`
	AppID             string                            // Deprecated but necessary for backward compatibility
}

type DynamicEventStub struct {
	ProjectID string `json:"project_id"`
	EventType string `json:"event_type" bson:"event_type" valid:"required~please provide an event type"`
	// Data is an arbitrary JSON value that gets sent as the body of the webhook to the endpoints
	Data          json.RawMessage   `json:"data" bson:"data" valid:"required~please provide your data"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

type Event struct {
	EndpointID string `json:"endpoint_id"`
	AppID      string `json:"app_id" bson:"app_id"` // Deprecated but necessary for backward compatibility
	EventType  string `json:"event_type" bson:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data          json.RawMessage   `json:"data" bson:"data" valid:"required~please provide your data"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

type FanoutEvent struct {
	OwnerID   string `json:"owner_id" valid:"required~please provide an owner id"`
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data          json.RawMessage   `json:"data" bson:"data" valid:"required~please provide your data"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

type IDs struct {
	IDs []string `json:"ids"`
}

type DeliveryAttempt struct {
	MessageID  string `json:"msg_id" bson:"msg_id"`
	APIVersion string `json:"api_version" bson:"api_version"`
	IPAddress  string `json:"ip" bson:"ip"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"created_at" bson:"created_at"`

	MessageResponse MessageResponse `json:"response" bson:"response"`
}

type MessageResponse struct {
	Status int             `json:"status" bson:"status"`
	Data   json.RawMessage `json:"data" bson:"data"`
}
type ExpireSecret struct {
	Secret     string `json:"secret"`
	Expiration int    `json:"expiration"`
}

type DashboardSummary struct {
	EventsSent   uint64                     `json:"events_sent" bson:"events_sent"`
	Applications int                        `json:"apps" bson:"apps"`
	Period       string                     `json:"period" bson:"period"`
	PeriodData   *[]datastore.EventInterval `json:"event_data,omitempty" bson:"event_data"`
}

type WebhookRequest struct {
	Event string          `json:"event" bson:"event"`
	Data  json.RawMessage `json:"data" bson:"data"`
}

type Subscription struct {
	Name       string `json:"name" bson:"name" valid:"required~please provide a valid subscription name"`
	SourceID   string `json:"source_id" bson:"source_id"`
	AppID      string `json:"app_id"` // Deprecated but necessary for backward compatibility
	EndpointID string `json:"endpoint_id" bson:"endpoint_id" valid:"required~please provide a valid endpoint id"`

	AlertConfig     *datastore.AlertConfiguration     `json:"alert_config,omitempty" bson:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration               `json:"retry_config,omitempty" bson:"retry_config,omitempty"`
	FilterConfig    *datastore.FilterConfiguration    `json:"filter_config,omitempty" bson:"filter_config,omitempty"`
	RateLimitConfig *datastore.RateLimitConfiguration `json:"rate_limit_config,omitempty" bson:"rate_limit_config,omitempty"`
}

type UpdateSubscription struct {
	Name       string `json:"name,omitempty"`
	AppID      string `json:"app_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	EndpointID string `json:"endpoint_id,omitempty"`

	AlertConfig     *datastore.AlertConfiguration     `json:"alert_config,omitempty"`
	RetryConfig     *RetryConfiguration               `json:"retry_config,omitempty"`
	FilterConfig    *datastore.FilterConfiguration    `json:"filter_config,omitempty"`
	RateLimitConfig *datastore.RateLimitConfiguration `json:"rate_limit_config,omitempty"`
}

type RetryConfiguration struct {
	Type            datastore.StrategyProvider `json:"type,omitempty" valid:"supported_retry_strategy~please provide a valid retry strategy type"`
	Duration        string                     `json:"duration,omitempty" valid:"duration~please provide a valid time duration"`
	IntervalSeconds uint64                     `json:"interval_seconds" valid:"int~please provide a valid interval seconds"`
	RetryCount      uint64                     `json:"retry_count" valid:"int~please provide a valid retry count"`
}

type CreateEndpointApiKey struct {
	Project    *datastore.Project
	Endpoint   *datastore.Endpoint
	Name       string `json:"name"`
	BaseUrl    string
	KeyType    datastore.KeyType `json:"key_type"`
	Expiration int               `json:"expiration"`
}

type PortalLink struct {
	Name               string   `json:"name" valid:"required~please provide the name field"`
	Endpoints          []string `json:"endpoints"`
	OwnerID            string   `json:"owner_id"`
	CanManageEndpoint bool     `json:"can_manage_endpoint"`
}

type PortalLinkResponse struct {
	UID                string                     `json:"uid"`
	Name               string                     `json:"name"`
	ProjectID          string                     `json:"project_id"`
	OwnerID            string                     `json:"owner_id"`
	Endpoints          []string                   `json:"endpoints"`
	EndpointCount      int                        `json:"endpoint_count"`
	CanManageEndpoint bool                       `json:"can_manage_endpoint"`
	Token              string                     `json:"token"`
	EndpointsMetadata  datastore.EndpointMetadata `json:"endpoints_metadata"`
	URL                string                     `json:"url"`
	CreatedAt          time.Time                  `json:"created_at,omitempty"`
	UpdatedAt          time.Time                  `json:"updated_at,omitempty"`
	DeletedAt          null.Time                  `json:"deleted_at,omitempty"`
}

type TestFilter struct {
	Request FilterSchema `json:"request"`
	Schema  FilterSchema `json:"schema"`
}

type FilterSchema struct {
	Headers interface{} `json:"header" bson:"header"`
	Body    interface{} `json:"body" bson:"body"`
}

// Generic function for looping over a slice of type M
// and returning a slice of type T
func NewListResponse[T, M any](items []M, fn func(item M) T) []T {
	results := make([]T, 0)

	for _, item := range items {
		results = append(results, fn(item))
	}

	return results
}
