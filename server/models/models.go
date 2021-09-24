package models

import (
	"encoding/json"
)

type Organisation struct {
	Name string `json:"name" bson:"name"`
}

type Application struct {
	OrgID   string `json:"org_id" bson:"org_id"`
	AppName string `json:"name" bson:"name"`
	Secret  string `json:"secret" bson:"secret"`
}

type Message struct {
	MessageID  string          `json:"msg_id" bson:"msg_id"`
	AppID      string          `json:"app_id" bson:"app_id"`
	EventType  string          `json:"event_type" bson:"event_type"`
	ProviderID string          `json:"provider_id" bson:"provider_id"`
	Data       json.RawMessage `json:"data" bson:"data"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"created_at" bson:"created_at"`
}

type MessageAttempt struct {
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
	URL         string `json:"url" bson:"url"`
	Secret      string `json:"secret" bson:"secret"`
	Description string `json:"description" bson:"description"`
	Disabled    bool   `json:"disabled" bson:"disabled"`
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
	MessagesSent uint64             `json:"messages_sent" bson:"messages_sent"`
	Applications int                `json:"apps" bson:"apps"`
	Period       string             `json:"period" bson:"period"`
	PeriodData   *[]MessageInterval `json:"message_data,omitempty" bson:"message_data"`
}

type MessageInterval struct {
	Data  MessageIntervalData `json:"data" bson:"_id"`
	Count uint64              `json:"count" bson:"count"`
}

type MessageIntervalData struct {
	Interval int64  `json:"index" bson:"index"`
	Time     string `json:"date" bson:"total_time"`
}

type WebhookRequest struct {
	Event string          `json:"event" bson:"event"`
	Data  json.RawMessage `json:"data" bson:"data"`
}
