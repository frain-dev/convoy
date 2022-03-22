package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Group struct {
	Name              string `json:"name" bson:"name" valid:"required~please provide a valid name"`
	LogoURL           string `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	RateLimit         int    `json:"rate_limit" bson:"rate_limit" valid:"int~please provide a valid rate limit,optional"`
	RateLimitDuration string `json:"rate_limit_duration" bson:"rate_limit_duration" valid:"int~please provide a valid rate limit duration,optional"`

	Config datastore.GroupConfig
}

type APIKey struct {
	Name      string            `json:"name"`
	Role      auth.Role         `json:"role"`
	Type      datastore.KeyType `json:"key_type"`
	ExpiresAt time.Time         `json:"expires_at"`
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
	CreatedAt time.Time `json:"created_at"`
}

type PortalAPIKeyResponse struct {
	Key     string    `json:"key"`
	Role    auth.Role `json:"role"`
	Url     string    `json:"url,omitempty"`
	Type    string    `json:"key_type"`
	AppID   string    `json:"app_id,omitempty"`
	GroupID string    `json:"group_id,omitempty"`
}

type Application struct {
	AppName      string `json:"name" bson:"name" valid:"required~please provide your appName"`
	SupportEmail string `json:"support_email" bson:"support_email" valid:"email~please provide a valid email"`
	IsDisabled   bool   `json:"is_disabled"`
}

type UpdateApplication struct {
	AppName      *string `json:"name" bson:"name" valid:"required~please provide your appName"`
	SupportEmail *string `json:"support_email" bson:"support_email" valid:"email~please provide a valid email"`
	IsDisabled   *bool   `json:"is_disabled"`
}

type Event struct {
	AppID     string `json:"app_id" bson:"app_id" valid:"required~please provide an app id"`
	EventType string `json:"event_type" bson:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" bson:"data" valid:"required~please provide your data"`
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

type Endpoint struct {
	URL               string   `json:"url" bson:"url"`
	Secret            string   `json:"secret" bson:"secret"`
	Description       string   `json:"description" bson:"description"`
	Events            []string `json:"events" bson:"events"`
	RateLimit         int      `json:"rate_limit" bson:"rate_limit"`
	RateLimitDuration string   `json:"rate_limit_duration" bson:"rate_limit_duration"`
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
