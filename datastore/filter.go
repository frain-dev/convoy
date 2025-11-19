package datastore

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Filter struct {
	Query           string
	OwnerID         string
	Project         *Project
	ProjectID       string
	EndpointID      string
	EndpointIDs     []string
	SubscriptionID  string
	EventID         string
	EventType       string
	SourceID        string
	SourceIDs       []string
	Pageable        Pageable
	IdempotencyKey  string
	BrokerMessageId string
	Status          []EventDeliveryStatus
	SearchParams    SearchParams
}

func (f *Filter) Scan(v interface{}) error {
	b, ok := v.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", v)
	}

	if string(b) == "null" {
		return nil
	}

	err := json.Unmarshal(b, &f)
	if err != nil {
		return err
	}

	return nil
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
	fmt.Fprintf(filterByBuilder, "project_id:=%s", f.ProjectID)
	fmt.Fprintf(filterByBuilder, " && created_at:[%d..%d]", f.SearchParams.CreatedAtStart, f.SearchParams.CreatedAtEnd)

	if len(f.EndpointID) > 0 {
		fmt.Fprintf(filterByBuilder, " && app_id:=%s", f.EndpointID)
	}

	if len(f.SourceID) > 0 {
		fmt.Fprintf(filterByBuilder, " && source_id:=%s", f.SourceID)
	}

	s = filterByBuilder.String()

	return s
}

type SearchFilter struct {
	Query    string
	FilterBy FilterBy
	Pageable Pageable
}
