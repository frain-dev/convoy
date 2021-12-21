package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/auth"
)

type Group struct {
	Name    string `json:"name" bson:"name"`
	LogoURL string `json:"logo_url" bson:"logo_url"`
	Config  convoy.GroupConfig
}

type APIKey struct {
	Role      auth.Role  `json:"role"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type APIKeyResponse struct {
	APIKey
	Key       string    `json:"key"`
	UID       string    `json:"uid"`
	CreatedAt time.Time `json:"created_at"`
}

type Application struct {
	AppName      string `json:"name" bson:"name"`
	SupportEmail string `json:"support_email" bson:"support_email"`
}

type Event struct {
	AppID     string `json:"app_id" bson:"app_id"`
	EventType string `json:"event_type" bson:"event_type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" bson:"data"`
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
	URL         string   `json:"url" bson:"url"`
	Secret      string   `json:"secret" bson:"secret"`
	Description string   `json:"description" bson:"description"`
	Events      []string `json:"events" bson:"events"`
}

type DashboardSummary struct {
	EventsSent   uint64                  `json:"events_sent" bson:"events_sent"`
	Applications int                     `json:"apps" bson:"apps"`
	Period       string                  `json:"period" bson:"period"`
	PeriodData   *[]convoy.EventInterval `json:"event_data,omitempty" bson:"event_data"`
}

type WebhookRequest struct {
	Event string          `json:"event" bson:"event"`
	Data  json.RawMessage `json:"data" bson:"data"`
}
