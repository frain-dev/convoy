package datastore

import (
	"encoding/json"
	"fmt"
)

type Filter struct {
	Query           string
	OwnerID         string
	UserID          string
	KeyType         KeyType
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

type FilterBy struct {
	OwnerID          string
	EndpointID       string
	EndpointIDs      []string
	SubscriptionName string
	ProjectID        string
	SourceID         string
	SearchParams     SearchParams
}
