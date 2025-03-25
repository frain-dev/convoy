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

	// JSONSchema is the JSON structure of the event type
	JSONSchema any `json:"json_schema"`
}

func (ce *CreateEventType) Validate() error {
	return util.Validate(ce)
}

type EventTypeResponse struct {
	*datastore.ProjectEventType
}

type UpdateEventType struct {
	// Category is a product-specific grouping for the event type
	Category string `json:"category"`

	// Description is used to describe what the event type does
	Description string `json:"description"`

	// JSONSchema is the JSON structure of the event type
	JSONSchema any `json:"json_schema"`
}

type ImportOpenAPISpec struct {
	Spec string `json:"spec"`
}
