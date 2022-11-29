package datastore

import (
	"fmt"
	"strings"
)

type Filter struct {
	Query           string
	Group           *Group
	AppID           string
	EventID         string
	SourceID        string
	SubscriptionIDs []string
	Pageable        Pageable
	Status          []EventDeliveryStatus
	SearchParams    SearchParams
}

type SourceFilter struct {
	Type     string
	Provider string
}

type ApiKeyFilter struct {
	GroupID string
	AppID   string
	UserID  string
	KeyType KeyType
}

type FilterBy struct {
	AppID        string
	GroupID      string
	SourceID     string
	SearchParams SearchParams
}

func (f *FilterBy) String() *string {
	var s string
	filterByBuilder := new(strings.Builder)
	filterByBuilder.WriteString(fmt.Sprintf("group_id:=%s", f.GroupID))
	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart, f.SearchParams.CreatedAtEnd))

	if len(f.AppID) > 0 {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_id:=%s", f.AppID))
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
