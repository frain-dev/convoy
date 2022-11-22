package datastore

import (
	"fmt"
	"strings"
)

type Filter struct {
	Query        string
	Group        *Group
	EndpointID   string
	EndpointIDs  []string
	EventID      string
	SourceID     string
	Pageable     Pageable
	Status       []EventDeliveryStatus
	SearchParams SearchParams
}

type SourceFilter struct {
	Type     string
	Provider string
}

type ApiKeyFilter struct {
	GroupID    string
	EndpointID string
	UserID     string
	KeyType    KeyType
}

type FilterBy struct {
	EndpointID   string
	EndpointIDs  []string
	GroupID      string
	SourceID     string
	SearchParams SearchParams
}

func (f *FilterBy) String() *string {
	var s string
	filterByBuilder := new(strings.Builder)
	filterByBuilder.WriteString(fmt.Sprintf("group_id:=%s", f.GroupID))
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
