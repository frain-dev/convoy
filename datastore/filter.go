package datastore

import (
	"fmt"
	"strings"
)

type Filter struct {
	Query          string
	OwnerID        string
	Project        *Project
	EndpointID     string
	EndpointIDs    []string
	SubscriptionID string
	EventID        string
	EventType      string
	SourceID       string
	Pageable       Pageable
	IdempotencyKey string
	Status         []EventDeliveryStatus
	SearchParams   SearchParams
}

type SourceFilter struct {
	Type     string
	Provider string
	Query    string
}

type ApiKeyFilter struct {
	ProjectID   string
	EndpointID  string
	EndpointIDs []string
	UserID      string
	KeyType     KeyType
}

type FilterBy struct {
	EndpointID   string
	EndpointIDs  []string
	ProjectID    string
	SourceID     string
	SearchParams SearchParams
}

func (f *FilterBy) String() *string {
	var s string
	filterByBuilder := new(strings.Builder)
	filterByBuilder.WriteString(fmt.Sprintf("project_id:=%s", f.ProjectID)) // TODO(daniel, RT): how to work around this?
	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart, f.SearchParams.CreatedAtEnd))

	if len(f.EndpointID) > 0 {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_id:=%s", f.EndpointID))
	}

	if len(f.SourceID) > 0 {
		filterByBuilder.WriteString(fmt.Sprintf(" && source_id:=%s", f.SourceID))
	}

	s = filterByBuilder.String()

	// we only return a pointer address here
	// because the typesense lib needs a string pointer
	return &s
}

type SearchFilter struct {
	Query    string
	FilterBy FilterBy
	Pageable Pageable
}

type EventDeliveryFilter struct {
	Sort           `json:"sort"`
	Pageable       `json:"pageable"`
	DateTimeFilter `json:"date_time_filter"`

	IdempotencyKeys []string              `json:"idempotency_keys"`
	SubscriptionIDs []string              `json:"subscription_ids"`
	EventID         string                `json:"event_id"`
	EndpointIDs     []string              `json:"endpoint_ids"`
	EventTypes      []string              `json:"event_types"`
	Status          []EventDeliveryStatus `json:"status"`
}

type DateTimeFilter struct {
	CreatedAtStart int64
	CreatedAtEnd   int64
}

type EventFilter struct {
	Query          string `json:"query"`
	IdempotencyKey string `json:"idempotency_key"`
	SourceID       string `json:"source_id"`

	Sort           `json:"sort"`
	Pageable       `json:"pageable"`
	DateTimeFilter `json:"date_time_filter"`
}
