package models

import (
	"encoding/json"
)

type Organisation struct {
	Name string `json:"name" bson:"name"`
}

type Application struct {
	OrgID   string `json:"orgId" bson:"orgId"`
	AppName string `json:"name" bson:"name"`
	Secret  string `json:"secret" bson:"secret"`
}

type Message struct {
	MessageID  string          `json:"msgId" bson:"msg_id"`
	AppID      string          `json:"appId" bson:"app_id"`
	EventType  string          `json:"eventType" bson:"event_type"`
	ProviderID string          `json:"providerId" bson:"provider_id"`
	Data       json.RawMessage `json:"data" bson:"data"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"createdAt" bson:"created_at"`
}

type MessageAttempt struct {
	MessageID  string `json:"msgId" bson:"msg_id"`
	APIVersion string `json:"apiVersion" bson:"api_version"`
	IPAddress  string `json:"ip" bson:"ip"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"createdAt" bson:"created_at"`

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
}

type Pageable struct {
	Page    int `json:"page" bson:"page"`
	PerPage int `json:"perPage" bson:"perPage"`
	Sort    int `json:"sort" bson:"sort"`
}

type SearchParams struct {
	CreatedAtStart int64 `json:"createdAtStart" bson:"created_at_start"`
	CreatedAtEnd   int64 `json:"createdAtEnd" bson:"created_at_end"`
}

type DashboardSummary struct {
	MessagesSent uint64             `json:"messagesSent" bson:"messages_sent"`
	Applications int                `json:"apps" bson:"apps"`
	Period       string             `json:"period" bson:"period"`
	PeriodData   *[]MessageInterval `json:"messageData,omitempty" bson:"message_data"`
}

type MessageInterval struct {
	Data  MessageIntervalData `json:"data" bson:"_id"`
	Count uint64              `json:"count" bson:"count"`
}

type MessageIntervalData struct {
	Interval int64  `json:"index" bson:"index"`
	Time     string `json:"date" bson:"total_time"`
}
