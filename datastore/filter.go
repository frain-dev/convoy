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
	SourceIDs      []string
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
	OwnerID          string
	EndpointID       string
	EndpointIDs      []string
	SubscriptionName string
	ProjectID        string
	SourceID         string
	SearchParams     SearchParams
}

func (f *FilterBy) String() string {
	var s string
	filterByBuilder := new(strings.Builder)
	// TODO(daniel, raymond): how to work around this?
	filterByBuilder.WriteString(fmt.Sprintf("project_id:=%s", f.ProjectID))
	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart, f.SearchParams.CreatedAtEnd))

	if len(f.EndpointID) > 0 {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_id:=%s", f.EndpointID))
	}

	if len(f.SourceID) > 0 {
		filterByBuilder.WriteString(fmt.Sprintf(" && source_id:=%s", f.SourceID))
	}

	s = filterByBuilder.String()

	return s
}

type SearchFilter struct {
	Query    string
	FilterBy FilterBy
	Pageable Pageable
}
