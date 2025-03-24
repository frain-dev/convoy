package converter

import (
	"fmt"
	"strings"

	"github.com/frain-dev/convoy/pkg/models"
	"github.com/getkin/kin-openapi/openapi3"
)

// Converter handles the conversion from OpenAPI spec to JSON Schema
type Converter struct {
	spec *openapi3.T
}

// New creates a new Converter instance
func New(spec *openapi3.T) *Converter {
	return &Converter{spec: spec}
}

// ExtractWebhooks extracts webhook schemas from OpenAPI spec
func (c *Converter) ExtractWebhooks(projectID string) (*models.WebhookCollection, error) {
	if c.spec.Paths == nil {
		return nil, fmt.Errorf("no paths found in OpenAPI spec")
	}

	collection := &models.WebhookCollection{
		ProjectID: projectID,
		Webhooks:  make([]models.WebhookSchema, 0),
	}

	// Look for webhook operations in paths
	paths := c.spec.Paths.Map()
	for path, pathItem := range paths {
		for method, operation := range pathItem.Operations() {
			// Check if this is a webhook endpoint (you might want to customize this logic)
			if isWebhook(path, method, operation) {
				if operation.RequestBody == nil || operation.RequestBody.Value == nil {
					continue
				}

				for contentType, mediaType := range operation.RequestBody.Value.Content {
					if !strings.Contains(contentType, "json") {
						continue
					}

					if mediaType.Schema == nil {
						continue
					}

					schema := c.convertSchema(mediaType.Schema.Value)
					webhookSchema := models.WebhookSchema{
						Name:        fmt.Sprintf("%s %s", method, path),
						Description: operation.Description,
						Schema:      schema,
					}

					collection.Webhooks = append(collection.Webhooks, webhookSchema)
				}
			}
		}
	}

	return collection, nil
}

// isWebhook determines if an operation is a webhook based on path, method, and operation details
func isWebhook(path, method string, operation *openapi3.Operation) bool {
	// You can customize this logic based on your OpenAPI spec conventions
	// For example, check if the path contains "webhook" or if there are specific tags
	if strings.Contains(strings.ToLower(path), "webhook") {
		return true
	}

	if operation.Tags != nil {
		for _, tag := range operation.Tags {
			if strings.Contains(strings.ToLower(tag), "webhook") {
				return true
			}
		}
	}

	// Check operation ID or summary
	if operation.OperationID != "" && strings.Contains(strings.ToLower(operation.OperationID), "webhook") {
		return true
	}

	if operation.Summary != "" && strings.Contains(strings.ToLower(operation.Summary), "webhook") {
		return true
	}

	return false
}

// convertSchema converts OpenAPI schema to JSON Schema
func (c *Converter) convertSchema(schema *openapi3.Schema) map[string]interface{} {
	result := make(map[string]interface{})

	// Add basic schema properties
	result["type"] = schema.Type
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Handle properties
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propSchema := range schema.Properties {
			properties[propName] = c.convertSchema(propSchema.Value)
		}
		result["properties"] = properties
	}

	// Handle required fields
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Handle array items
	if schema.Items != nil && schema.Items.Value != nil {
		result["items"] = c.convertSchema(schema.Items.Value)
	}

	// Handle additional properties
	if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		result["additionalProperties"] = c.convertSchema(schema.AdditionalProperties.Schema.Value)
	}

	// Handle enums
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	// Handle format
	if schema.Format != "" {
		result["format"] = schema.Format
	}

	return result
}
