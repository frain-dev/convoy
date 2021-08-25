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
}

type Message struct {
	MessageID  string          `json:"msgId" bson:"msg_id"`
	AppID      string          `json:"appId" bson:"app_id"`
	EventType  string          `json:"eventType" bson:"event_type"`
	ProviderID string          `json:"providerId" bson:"provider_id"`
	Data       json.RawMessage `json:"data" bson:"data"`

	Status    string `json:"status" bson:"status"`
	CreatedAt int64  `json:"createdAt" bson:"created_at"`

	NextSendTime int64          `json:"nextSendTime"`
	AttemptCount int64          `json:"attemptCount"`
	LastAttempt  MessageAttempt `json:"lastAttempt"`
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
}

type SearchParams struct {
	CreatedAtStart int64 `json:"createdAtStart" bson:"created_at_start"`
	CreatedAtEnd   int64 `json:"createdAtEnd" bson:"created_at_end"`
}

type DashboardSummary struct {
	MessagesSent int        `json:"messages" bson:"messages"`
	Applications int        `json:"apps" bson:"apps"`
	Hourly       *[]Hourly  `json:"hourly,omitempty" bson:"hourly"`
	Daily        *[]Daily   `json:"daily,omitempty" bson:"daily"`
	Monthly      *[]Monthly `json:"monthly,omitempty" bson:"monthly"`
	Yearly       *[]Yearly  `json:"yearly,omitempty" bson:"yearly"`
}

type Hourly struct {
	Hour  int `json:"hour" bson:"hour"`
	Count int `json:"count" bson:"count"`
}
type Daily struct {
	Day   int `json:"day" bson:"day"`
	Count int `json:"count" bson:"count"`
}
type Monthly struct {
	Month int `json:"month" bson:"month"`
	Count int `json:"count" bson:"count"`
}
type Yearly struct {
	Year  int `json:"year" bson:"year"`
	Count int `json:"count" bson:"count"`
}
