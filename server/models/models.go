package models

import (
	"encoding/json"

	"github.com/frain-dev/convoy/config"
)

type Group struct {
	Name    string `json:"name" bson:"name" valid:"required~please provide a valid name"`
	LogoURL string `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	Config  config.GroupConfig
}

type Application struct {
	AppName      string `json:"name" bson:"name" valid:"required~please provide your appName"`
	SupportEmail string `json:"support_email" bson:"support_email" valid:"email~please provide a valid email"`
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
	URL         string   `json:"url" bson:"url"`
	Secret      string   `json:"secret" bson:"secret"`
	Description string   `json:"description" bson:"description"`
	Events      []string `json:"events" bson:"events"`
}

type Pageable struct {
	Page    int `json:"page" bson:"page"`
	PerPage int `json:"per_page" bson:"per_page"`
	Sort    int `json:"sort" bson:"sort"`
}

type SearchParams struct {
	CreatedAtStart int64 `json:"created_at_start" bson:"created_at_start"`
	CreatedAtEnd   int64 `json:"created_at_end" bson:"created_at_end"`
}

type DashboardSummary struct {
	EventsSent   uint64           `json:"events_sent" bson:"events_sent"`
	Applications int              `json:"apps" bson:"apps"`
	Period       string           `json:"period" bson:"period"`
	PeriodData   *[]EventInterval `json:"event_data,omitempty" bson:"event_data"`
}

type EventInterval struct {
	Data  EventIntervalData `json:"data" bson:"_id"`
	Count uint64            `json:"count" bson:"count"`
}

type EventIntervalData struct {
	Interval int64  `json:"index" bson:"index"`
	Time     string `json:"date" bson:"total_time"`
}

type WebhookRequest struct {
	Event string          `json:"event" bson:"event"`
	Data  json.RawMessage `json:"data" bson:"data"`
}
