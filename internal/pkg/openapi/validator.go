package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// ValidationResult represents the result of schema validation
type ValidationResult struct {
	IsValid bool              `json:"is_valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// Validate validates the given data against the webhook's JSON schema
func (w *Webhook) Validate(data interface{}) (*ValidationResult, error) {
	// Convert schema to JSON
	schemaBytes, err := json.Marshal(w.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %v", err)
	}

	// Load schema
	schemaLoader := gojsonschema.NewStringLoader(string(schemaBytes))

	// Load data
	var documentLoader gojsonschema.JSONLoader
	switch v := data.(type) {
	case string:
		documentLoader = gojsonschema.NewStringLoader(v)
	case []byte:
		documentLoader = gojsonschema.NewStringLoader(string(v))
	default:
		// For other types, marshal to JSON first
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %v", err)
		}
		documentLoader = gojsonschema.NewStringLoader(string(dataBytes))
	}

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	// Convert validation result
	validationResult := &ValidationResult{
		IsValid: result.Valid(),
	}

	if !result.Valid() {
		validationResult.Errors = make([]ValidationError, 0, len(result.Errors()))
		for _, err := range result.Errors() {
			validationResult.Errors = append(validationResult.Errors, ValidationError{
				Field:       err.Field(),
				Description: err.Description(),
			})
		}
	}

	return validationResult, nil
}
