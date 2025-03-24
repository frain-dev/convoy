package openapi

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type Webhook struct {
	Name        string           `json:"name"`
	ProjectID   string           `json:"project_id"`
	Description string           `json:"description"`
	Schema      *openapi3.Schema `json:"schema"`
}

type Collection struct {
	ProjectID string    `json:"project_id"`
	Webhooks  []Webhook `json:"webhooks"`
}

// Converter handles the conversion from OpenAPI spec to JSON Schema
type Converter struct {
	doc *openapi3.T
}

// New creates a new Converter instance
func New(doc interface{}) (*Converter, error) {
	if doc == nil {
		return nil, fmt.Errorf("OpenAPI document is nil")
	}

	t, ok := doc.(*openapi3.T)
	if !ok {
		return nil, fmt.Errorf("unsupported OpenAPI document type")
	}

	return &Converter{doc: t}, nil
}

// ExtractWebhooks extracts webhook schemas from OpenAPI spec
func (c *Converter) ExtractWebhooks(projectID string) (*Collection, error) {
	collection := &Collection{
		ProjectID: projectID,
		Webhooks:  make([]Webhook, 0),
	}

	// Try official webhooks field first (OpenAPI 3.1)
	if c.doc.Extensions != nil {
		if webhooksExt, ok := c.doc.Extensions["webhooks"]; ok {
			webhooksMap, ok := webhooksExt.(map[string]interface{})
			if ok {
				for name, pathItemRaw := range webhooksMap {
					webhook, err := c.extractWebhook(name, pathItemRaw, projectID)
					if err == nil {
						collection.Webhooks = append(collection.Webhooks, webhook)
					}
				}
			}
		}
	}

	// If no webhooks found, try x-webhooks extension (OpenAPI 3.0)
	if len(collection.Webhooks) == 0 && c.doc.Extensions != nil {
		if webhooksExt, ok := c.doc.Extensions["x-webhooks"]; ok {
			webhooksMap, ok := webhooksExt.(map[string]interface{})
			if ok {
				for name, pathItemRaw := range webhooksMap {
					webhook, err := c.extractWebhook(name, pathItemRaw, projectID)
					if err == nil {
						collection.Webhooks = append(collection.Webhooks, webhook)
					}
				}
			}
		}
	}

	if len(collection.Webhooks) == 0 {
		return nil, fmt.Errorf("no webhooks found in OpenAPI spec")
	}

	return collection, nil
}

// extractWebhook extracts a single webhook from a path item
func (c *Converter) extractWebhook(name string, pathItemRaw interface{}, projectID string) (Webhook, error) {
	webhook := Webhook{
		Name:      name,
		ProjectID: projectID,
	}

	pathItemMap, ok := pathItemRaw.(map[string]interface{})
	if !ok {
		return webhook, fmt.Errorf("invalid path item format")
	}

	postOp, ok := pathItemMap["post"].(map[string]interface{})
	if !ok {
		return webhook, fmt.Errorf("no POST operation found")
	}

	if desc, ok := postOp["description"].(string); ok {
		webhook.Description = desc
	}

	if reqBody, ok := postOp["requestBody"].(map[string]interface{}); ok {
		if content, ok := reqBody["content"].(map[string]interface{}); ok {
			if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
				if schema, ok := jsonContent["schema"].(map[string]interface{}); ok {
					if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
						schemaName := ref[len("#/components/schemas/"):]
						if c.doc.Components != nil && c.doc.Components.Schemas != nil {
							if schema, ok := c.doc.Components.Schemas[schemaName]; ok {
								webhook.Schema = schema.Value
							}
						}
					}
				}
			}
		}
	}

	if webhook.Schema == nil {
		return webhook, fmt.Errorf("no schema found")
	}

	return webhook, nil
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
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
