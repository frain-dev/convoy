package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Project struct {
	Name              string                   `json:"name" bson:"name" valid:"required~please provide a valid name"`
	Type              datastore.ProjectType    `json:"type" bson:"type" valid:"required~please provide a valid type,in(incoming|outgoing)"`
	LogoURL           string                   `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	RateLimit         int                      `json:"rate_limit" bson:"rate_limit" valid:"int~please provide a valid rate limit,optional"`
	RateLimitDuration string                   `json:"rate_limit_duration" bson:"rate_limit_duration" valid:"alphanum~please provide a valid rate limit duration,optional"`
	Config            *datastore.ProjectConfig `json:"config"`
}

type UpdateProject struct {
	Name              string                   `json:"name" bson:"name" valid:"required~please provide a valid name"`
	LogoURL           string                   `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	RateLimit         int                      `json:"rate_limit" bson:"rate_limit" valid:"int~please provide a valid rate limit,optional"`
	RateLimitDuration string                   `json:"rate_limit_duration" bson:"rate_limit_duration" valid:"alphanum~please provide a valid rate limit duration,optional"`
	Config            *datastore.ProjectConfig `json:"config" valid:"optional"`
}

type Organisation struct {
	Name         string `json:"name" bson:"name"`
	CustomDomain string `json:"custom_domain" bson:"custom_domain"`
}

type Configuration struct {
	IsAnalyticsEnabled *bool                                 `json:"is_analytics_enabled"`
	IsSignupEnabled    *bool                                 `json:"is_signup_enabled"`
	StoragePolicy      *datastore.StoragePolicyConfiguration `json:"storage_policy"`
}

type ConfigurationResponse struct {
	UID                string                                `json:"uid"`
	IsAnalyticsEnabled bool                                  `json:"is_analytics_enabled"`
	IsSignupEnabled    bool                                  `json:"is_signup_enabled"`
	ApiVersion         string                                `json:"api_version"`
	StoragePolicy      *datastore.StoragePolicyConfiguration `json:"storage_policy"`

	CreatedAt primitive.DateTime  `json:"created_at,omitempty"`
	UpdatedAt primitive.DateTime  `json:"updated_at,omitempty"`
	DeletedAt *primitive.DateTime `json:"deleted_at,omitempty"`
}

type OrganisationInvite struct {
	InviteeEmail string    `json:"invitee_email" valid:"required~please provide a valid invitee email,email"`
	Role         auth.Role `json:"role" bson:"role"`
}

type APIKey struct {
	Name      string            `json:"name"`
	Role      Role              `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt time.Time         `json:"expires_at"`
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
	UID       string             `json:"uid"`
	Name      string             `json:"name"`
	Role      auth.Role          `json:"role"`
	Type      datastore.KeyType  `json:"key_type"`
	ExpiresAt primitive.DateTime `json:"expires_at,omitempty"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty"`
}

type APIKeyResponse struct {
	APIKey
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateProjectResponse struct {
	APIKey  *APIKeyResponse    `json:"api_key"`
	Project *datastore.Project `json:"project"`
}

type PortalAPIKeyResponse struct {
	Key        string    `json:"key"`
	Role       auth.Role `json:"role"`
	Url        string    `json:"url,omitempty"`
	Type       string    `json:"key_type"`
	EndpointID string    `json:"endpoint_id,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
}

type SourceResponse struct {
	UID            string                    `json:"uid"`
	MaskID         string                    `json:"mask_id"`
	ProjectID      string                    `json:"project_id"`
	Name           string                    `json:"name"`
	Type           datastore.SourceType      `json:"type"`
	URL            string                    `json:"url"`
	IsDisabled     bool                      `json:"is_disabled"`
	Verifier       *datastore.VerifierConfig `json:"verifier"`
	Provider       datastore.SourceProvider  `json:"provider"`
	ProviderConfig *datastore.ProviderConfig `json:"provider_config"`

	CreatedAt primitive.DateTime  `json:"created_at,omitempty"`
	UpdatedAt primitive.DateTime  `json:"updated_at,omitempty"`
	DeletedAt *primitive.DateTime `json:"deleted_at,omitempty"`
}

type LoginUser struct {
	Username string `json:"username" valid:"required~please provide your username"`
	Password string `json:"password" valid:"required~please provide your password"`
}

type RegisterUser struct {
	FirstName        string `json:"first_name" valid:"required~please provide a first name"`
	LastName         string `json:"last_name" valid:"required~please provide a last name"`
	Email            string `json:"email" valid:"required~please provide an email,email"`
	Password         string `json:"password" valid:"required~please provide a password"`
	OrganisationName string `json:"org_name" valid:"required~please provide an organisation name"`
}

type LoginUserResponse struct {
	UID           string `json:"uid"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	Token         Token  `json:"token"`
	EmailVerified bool   `json:"email_verified"`

	CreatedAt primitive.DateTime  `json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt primitive.DateTime  `json:"updated_at,omitempty" bson:"updated_at"`
	DeletedAt *primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at"`
}

type UserInviteTokenResponse struct {
	Token *datastore.OrganisationInvite `json:"token"`
	User  *datastore.User               `json:"user"`
}

type Token struct {
	AccessToken  string `json:"access_token" valid:"required~please provide an access token"`
	RefreshToken string `json:"refresh_token" valid:"required~please provide a refresh token"`
}

type User struct {
	FirstName string    `json:"first_name" valid:"required~please provide a first name"`
	LastName  string    `json:"last_name" valid:"required~please provide a last name"`
	Email     string    `json:"email" valid:"required~please provide an email,email"`
	Password  string    `json:"password" valid:"required~please provide a password"`
	Role      auth.Role `json:"role" bson:"role"`
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

type Source struct {
	Name       string                   `json:"name" valid:"required~please provide a source name"`
	Type       datastore.SourceType     `json:"type" valid:"required~please provide a type,supported_source~unsupported source type"`
	Provider   datastore.SourceProvider `json:"provider"`
	IsDisabled bool                     `json:"is_disabled"`
	Verifier   datastore.VerifierConfig `json:"verifier" valid:"required~please provide a verifier"`
}

type UpdateSource struct {
	Name           *string                  `json:"name" valid:"required~please provide a source name"`
	Type           datastore.SourceType     `json:"type" valid:"required~please provide a type,supported_source~unsupported source type"`
	IsDisabled     *bool                    `json:"is_disabled"`
	ForwardHeaders []string                 `json:"forward_headers"`
	Verifier       datastore.VerifierConfig `json:"verifier" valid:"required~please provide a verifier"`
}

type Event struct {
	EndpointID string `json:"endpoint_id"`
	AppID      string `json:"app_id" bson:"app_id"`
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
	DisableEndpoint *bool                             `json:"disable_endpoint" bson:"disable_endpoint"`
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
	DisableEndpoint *bool                             `json:"disable_endpoint" bson:"disable_endpoint"`
}

type RetryConfiguration struct {
	Type            datastore.StrategyProvider `json:"type,omitempty" valid:"supported_retry_strategy~please provide a valid retry strategy type"`
	Duration        string                     `json:"duration,omitempty" valid:"duration~please provide a valid time duration"`
	IntervalSeconds uint64                     `json:"interval_seconds" valid:"int~please provide a valid interval seconds"`
	RetryCount      uint64                     `json:"retry_count" valid:"int~please provide a valid retry count"`
}

type UpdateUser struct {
	FirstName string `json:"first_name" valid:"required~please provide a first name"`
	LastName  string `json:"last_name" valid:"required~please provide a last name"`
	Email     string `json:"email" valid:"required~please provide an email,email"`
}

type UpdatePassword struct {
	CurrentPassword      string `json:"current_password" valid:"required~please provide the current password"`
	Password             string `json:"password" valid:"required~please provide the password field"`
	PasswordConfirmation string `json:"password_confirmation" valid:"required~please provide the password confirmation field"`
}

type UserExists struct {
	Email string `json:"email" valid:"required~please provide an email,email"`
}

type ForgotPassword struct {
	Email string `json:"email" valid:"required~please provide an email,email"`
}

type ResetPassword struct {
	Password             string `json:"password" valid:"required~please provide the password field"`
	PasswordConfirmation string `json:"password_confirmation" valid:"required~please provide the password confirmation field"`
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
	Name      string   `json:"name" valid:"required~please provide the name field"`
	Endpoints []string `json:"endpoints"`
}

type PortalLinkResponse struct {
	UID               string               `json:"uid"`
	Name              string               `json:"name"`
	ProjectID         string               `json:"project_id"`
	Endpoints         []string             `json:"endpoints"`
	EndpointCount     int                  `json:"endpoint_count"`
	Token             string               `json:"token"`
	EndpointsMetadata []datastore.Endpoint `json:"endpoints_metadata"`
	URL               string               `json:"url"`
	CreatedAt         primitive.DateTime   `json:"created_at,omitempty"`
	UpdatedAt         primitive.DateTime   `json:"updated_at,omitempty"`
	DeletedAt         primitive.DateTime   `json:"deleted_at,omitempty"`
}

type TestFilter struct {
	Request map[string]interface{} `json:"request"`
	Schema  map[string]interface{} `json:"schema"`
}
