package models

import (
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

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateFilterRequest represents the request to create a filter
type CreateFilterRequest struct {
	// Type of event this filter applies to (required)
	EventType string `json:"event_type" validate:"required"`

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`
}

// UpdateFilterRequest represents the request to update a filter
type UpdateFilterRequest struct {
	// Type of event this filter applies to (optional)
	EventType string `json:"event_type"`

	// Header matching criteria (optional)
	Headers datastore.M `json:"headers"`

	// Body matching criteria (optional)
	Body datastore.M `json:"body"`

	// Whether the filter uses flattened JSON paths (optional)
	IsFlattened *bool `json:"is_flattened"`
}

// TestFilterRequest represents the request to test a filter
type TestFilterRequest struct {
	// Sample payload to test against the filter (required)
	Payload interface{} `json:"payload" validate:"required"`
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
