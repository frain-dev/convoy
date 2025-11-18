package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

type Webhook struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Schema      *openapi3.Schema `json:"schema"`
}

func (w *Webhook) AsBytes() []byte {
	if w.Schema == nil {
		return nil
	}

	// Create a map with the schema type
	schemaMap := map[string]interface{}{
		"type":       "object",
		"properties": w.Schema.Properties,
		"required":   w.Schema.Required,
	}

	bytes, err := json.Marshal(schemaMap)
	if err != nil {
		return nil
	}

	return bytes
}

type Collection struct {
	Webhooks map[string]*Webhook `json:"webhooks"`
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

	var docV3 *openapi3.T

	// Try to convert from OpenAPI 2.0 if needed
	if docV2, ok := doc.(*openapi2.T); ok {
		var err error
		docV3, err = openapi2conv.ToV3(docV2)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAPI 2.0 to 3.0: %v", err)
		}
	} else if docV3, ok = doc.(*openapi3.T); !ok {
		return nil, fmt.Errorf("unsupported OpenAPI document type")
	}

	return &Converter{doc: docV3}, nil
}

// NewFromBytes creates a new Converter instance from bytes
func NewFromBytes(data []byte) (*Converter, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	swagger, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("error loading OpenAPI spec: %v", err)
	}

	// Create converter
	return New(swagger)
}

// ExtractWebhooks extracts webhook schemas from OpenAPI spec
func (c *Converter) ExtractWebhooks() (*Collection, error) {
	collection := &Collection{
		Webhooks: make(map[string]*Webhook),
	}

	// Try the official webhooks field first (OpenAPI 3.1)
	if c.doc.Extensions != nil {
		// Try both webhooks and x-webhooks
		for _, key := range []string{"webhooks", "x-webhooks"} {
			if webhooksExt, ok := c.doc.Extensions[key]; ok {
				webhooksMap, ok := webhooksExt.(map[string]interface{})
				if ok {
					for name, pathItemRaw := range webhooksMap {
						webhook, err := c.extractWebhook(pathItemRaw)
						if err == nil {
							webhook.Name = name
							collection.Webhooks[name] = webhook
						}
					}
				}
			}
		}
	}

	// If still no webhooks found, try to find them in paths (OpenAPI 2.0 style)
	if len(collection.Webhooks) == 0 && c.doc.Paths != nil {
		for path, pathItem := range c.doc.Paths.Map() {
			if pathItem != nil && pathItem.Post != nil && isWebhook(path, pathItem.Post) {
				if pathItem.Post.RequestBody != nil && pathItem.Post.RequestBody.Value != nil &&
					pathItem.Post.RequestBody.Value.Content != nil &&
					pathItem.Post.RequestBody.Value.Content["application/json"] != nil &&
					pathItem.Post.RequestBody.Value.Content["application/json"].Schema != nil {
					name := extractWebhookName(path)
					schema := pathItem.Post.RequestBody.Value.Content["application/json"].Schema.Value
					if schema != nil {
						// Create a copy of the schema to avoid modifying the original
						schemaCopy := *schema

						// Add examples from the request body if available
						if pathItem.Post.RequestBody.Value.Content["application/json"].Example != nil {
							schemaCopy.Example = pathItem.Post.RequestBody.Value.Content["application/json"].Example
						}

						webhook := &Webhook{
							Name:        name,
							Description: pathItem.Post.Description,
							Schema:      &schemaCopy,
						}
						collection.Webhooks[name] = webhook
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

// extractWebhook extracts a webhook from a path item
func (c *Converter) extractWebhook(pathItemRaw interface{}) (*Webhook, error) {
	pathItemMap, ok := pathItemRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid path item format")
	}

	postOp, ok := pathItemMap["post"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no POST operation found")
	}

	description := ""
	if desc, ok := postOp["description"].(string); ok {
		description = desc
	}

	if reqBody, ok := postOp["requestBody"].(map[string]interface{}); ok {
		if content, ok := reqBody["content"].(map[string]interface{}); ok {
			if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
				if schema, ok := jsonContent["schema"].(map[string]interface{}); ok {
					// Create a new schema
					newSchema := &openapi3.Schema{}

					// Copy properties
					if props, ok := schema["properties"].(map[string]interface{}); ok {
						newSchema.Properties = make(map[string]*openapi3.SchemaRef)
						for propName, propValue := range props {
							propMap, ok := propValue.(map[string]interface{})
							if !ok {
								continue
							}

							propSchema := &openapi3.Schema{}
							if propType, ok := propMap["type"].(string); ok {
								types := openapi3.Types{propType}
								propSchema.Type = &types
							}
							if format, ok := propMap["format"].(string); ok {
								propSchema.Format = format
							}
							if enum, ok := propMap["enum"].([]interface{}); ok {
								propSchema.Enum = enum
							}
							if minimum, ok := propMap["minimum"].(float64); ok {
								propSchema.Min = &minimum
							}
							// Extract field description
							if desc, ok := propMap["description"].(string); ok {
								propSchema.Description = desc
							}

							newSchema.Properties[propName] = &openapi3.SchemaRef{
								Value: propSchema,
							}
						}
					}

					// Copy required fields
					if required, ok := schema["required"].([]interface{}); ok {
						newSchema.Required = make([]string, len(required))
						for i, r := range required {
							if str, ok := r.(string); ok {
								newSchema.Required[i] = str
							}
						}
					}

					// Copy example
					if example, ok := schema["example"].(map[string]interface{}); ok {
						newSchema.Example = example
					}

					return &Webhook{
						Description: description,
						Schema:      newSchema,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no schema found")
}

// isWebhook determines if an operation is a webhook based on the path and operation details
func isWebhook(path string, operation *openapi3.Operation) bool {
	// You can customize this logic based on your OpenAPI spec conventions,
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

// extractWebhookName extracts a webhook name from a path
func extractWebhookName(path string) string {
	// Remove any leading/trailing slashes
	path = strings.Trim(path, "/")

	// Split the path into segments
	segments := strings.Split(path, "/")

	// Find the segment containing "webhook"
	for i, segment := range segments {
		if strings.Contains(strings.ToLower(segment), "webhook") {
			// If this is the last segment, use it
			if i == len(segments)-1 {
				return segment
			}
			// Otherwise, use the next segment if it exists
			if i < len(segments)-1 {
				return segments[i+1]
			}
			return segment
		}
	}

	// If no webhook segment found, use the last segment
	if len(segments) > 0 {
		return segments[len(segments)-1]
	}

	// Fallback to a generic name
	return "webhook"
}
