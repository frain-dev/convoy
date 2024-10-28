package models

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type CreateEventType struct {
	// Name is the event type name. E.g., invoice.created
	Name string `json:"name" valid:"required~please provide a name for this event type"`

	// Category is a product-specific grouping for the event type
	Category string `json:"category"`

	// Description is used to describe what the event type does
	Description string `json:"description"`
}

func (ce *CreateEventType) Validate() error {
	return util.Validate(ce)
}

type EventTypeResponse struct {
	EventType *datastore.ProjectEventType `json:"event_type"`
}

type EventTypeListResponse struct {
	EventTypes []datastore.ProjectEventType `json:"event_types"`
}

type UpdateEventType struct {
	// Category is a product-specific grouping for the event type
	Category string `json:"category"`

	// Description is used to describe what the event type does
	Description string `json:"description"`
}
