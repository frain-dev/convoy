package models

import (
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/openapi"
	"github.com/frain-dev/convoy/util"
	"github.com/getkin/kin-openapi/openapi3"
)

type CreateEventType struct {
	// Name is the event type name. E.g., invoice.created
	Name string `json:"name" valid:"required~please provide a name for this event type"`

	// Category is a product-specific grouping for the event type
	Category string `json:"category"`

	// Description is used to describe what the event type does
	Description string `json:"description"`

	// JSONSchema is the JSON structure of the event type
	JSONSchema map[string]interface{} `json:"json_schema"`
}

func (ce *CreateEventType) Validate() error {
	if err := util.Validate(ce); err != nil {
		return err
	}

	if ce.JSONSchema != nil {
		if err := validateJSONSchema(ce.JSONSchema); err != nil {
			return fmt.Errorf("invalid JSON schema: %w", err)
		}
	}

	return nil
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
	JSONSchema map[string]interface{} `json:"json_schema"`
}

func (ue *UpdateEventType) Validate() error {
	if err := util.Validate(ue); err != nil {
		return err
	}

	if ue.JSONSchema != nil {
		if err := validateJSONSchema(ue.JSONSchema); err != nil {
			return fmt.Errorf("invalid JSON schema: %w", err)
		}
	}

	return nil
}

type ImportOpenAPISpec struct {
	Spec string `json:"spec"`
}

// validateJSONSchema validates that the provided schema is a valid JSON Schema
func validateJSONSchema(schema interface{}) error {
	// Convert schema to JSON
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %v", err)
	}

	// Create a webhook with the schema for validation
	webhook := &openapi.Webhook{
		Schema: &openapi3.Schema{},
	}

	// Unmarshal the schema into the webhook's Schema field
	if err = webhook.Schema.UnmarshalJSON(schemaBytes); err != nil {
		return fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	// Validate the schema
	result, err := webhook.ValidateSchema()
	if err != nil {
		return fmt.Errorf("schema validation failed: %v", err)
	}

	if !result.IsValid {
		var errors []string
		for _, validationError := range result.Errors {
			errors = append(errors, fmt.Sprintf("%s: %s", validationError.Field, validationError.Description))
		}
		return fmt.Errorf("invalid JSON schema: %v", errors)
	}

	return nil
}
