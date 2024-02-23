package models

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type AddEventToCatalogue struct {
	// EventID for the event to be added to the event catalogue
	EventID string `json:"event_id" valid:"required~please provide an event id"`

	// The name of this event in the catalogue e.g. invoice.paid
	Name string `json:"name" valid:"required~please provide a name for this event type"`
}

func (ds *AddEventToCatalogue) Validate() error { return util.Validate(ds) }

type CatalogueOpenAPISpec struct {
	// An openapi 3.0+ specification in YAML format. Convoy use the webhook section of the specification.
	// to render the event catalogue. See https://github.com/OAI/OpenAPI-Specification/blob/main/examples/v3.1/webhook-example.yaml
	// https://redocly.com/blog/document-webhooks-with-openapi/
	OpenAPISpec []byte `json:"open_api_spec" valid:"required~please provide an openapi spec"`
}

func (ds *CatalogueOpenAPISpec) Validate() error { return util.Validate(ds) }

type UpdateCatalogue struct {
	Events      datastore.EventDataCatalogues
	OpenAPISpec []byte `json:"open_api_spec"`
}
