package models

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/datastore"
)

// Filter represents a filter entity in the API
type Filter struct {
	// Unique identifier for the filter
	UID string `json:"uid"`

	// ID of the subscription this filter belongs to
	SubscriptionID string `json:"subscription_id"`

	// Type of event this filter applies to
	EventType string `json:"event_type"`

	// Non-null when this filter is active.
	EnabledAt *time.Time `json:"enabled_at"`

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`

	// Query matching criteria (optional)
	Query datastore.M `json:"query"`

	// Path matching criteria (optional)
	Path datastore.M `json:"path"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateFilterRequest represents the request to create a filter
type CreateFilterRequest struct {
	// Type of event this filter applies to (required)
	EventType string `json:"event_type" validate:"required"`

	// Non-null when this filter is active. Defaults to now when omitted.
	EnabledAt OptionalTime `json:"enabled_at,omitempty,omitzero"`

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`

	// Query matching criteria (optional)
	Query datastore.M `json:"query"`

	// Path matching criteria (optional)
	Path datastore.M `json:"path"`
}

// UpdateFilterRequest represents the request to update a filter
type UpdateFilterRequest struct {
	// Type of event this filter applies to (optional)
	EventType string `json:"event_type"`

	// Non-null when this filter is active.
	EnabledAt OptionalTime `json:"enabled_at,omitempty,omitzero"`

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`

	// Query matching criteria (optional)
	Query datastore.M `json:"query"`

	// Path matching criteria (optional)
	Path datastore.M `json:"path"`

	// Whether the filter uses flattened JSON paths (optional)
	IsFlattened *bool `json:"is_flattened"`
}

// TestFilterRequest represents the request to test a filter
type TestFilterRequest struct {
	// Sample payload to test against body filter rules. Optional when request scopes are supplied.
	Payload interface{} `json:"payload,omitempty"`

	// Request scopes to test against the filter.
	Request TestFilterRequestScopes `json:"request,omitempty"`
}

type TestFilterRequestScopes struct {
	Body interface{} `json:"body,omitempty"`

	// Headers accepts either "headers" or "header" for compatibility with the subscription filter tester.
	Headers datastore.M `json:"headers,omitempty"`
	Header  datastore.M `json:"header,omitempty"`
	Query   datastore.M `json:"query,omitempty"`
	Path    datastore.M `json:"path,omitempty"`
}

func (tf TestFilterRequest) Transform() datastore.FilterTestRequest {
	headers := tf.Request.Headers
	if len(headers) == 0 {
		headers = tf.Request.Header
	}

	body := tf.Request.Body
	if body == nil {
		body = tf.Payload
	}

	return datastore.FilterTestRequest{
		Body:    body,
		Headers: headers,
		Query:   tf.Request.Query,
		Path:    tf.Request.Path,
	}
}

// FilterResponse represents the response for a single filter
type FilterResponse struct {
	*datastore.EventTypeFilter
}

// TestFilterResponse represents the response for a filter test
type TestFilterResponse struct {
	// Whether the payload matches the filter criteria
	IsMatch bool `json:"is_match"`
}

// BulkUpdateFilterRequest is a request to update a filter in bulk
type BulkUpdateFilterRequest struct {
	UID       string                 `json:"uid" validate:"required"`
	EventType string                 `json:"event_type,omitempty"`
	EnabledAt OptionalTime           `json:"enabled_at,omitempty,omitzero"`
	Headers   map[string]interface{} `json:"headers,omitempty"`
	Body      map[string]interface{} `json:"body,omitempty"`
	Query     map[string]interface{} `json:"query,omitempty"`
	Path      map[string]interface{} `json:"path,omitempty"`
}

type OptionalTime struct {
	Set  bool
	Time *time.Time
}

func (o *OptionalTime) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Time = nil
		return nil
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	o.Time = &t
	return nil
}

func (o OptionalTime) MarshalJSON() ([]byte, error) {
	if o.Time == nil {
		return []byte("null"), nil
	}

	return json.Marshal(o.Time)
}

func (o OptionalTime) IsZero() bool {
	return !o.Set && o.Time == nil
}
