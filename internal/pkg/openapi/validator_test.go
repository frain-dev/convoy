package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestWebhook_ValidateSchema(t *testing.T) {
	tests := []struct {
		name          string
		schema        *openapi3.Schema
		wantValid     bool
		wantErrorLen  int
		wantErrorDesc string
	}{
		{
			name: "Valid schema",
			schema: &openapi3.Schema{
				Type:     &openapi3.Types{"object"},
				Required: []string{"event_type", "timestamp"},
				Properties: map[string]*openapi3.SchemaRef{
					"event_type": {
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
							Enum: []interface{}{"created", "updated", "deleted"},
						},
					},
					"timestamp": {
						Value: &openapi3.Schema{
							Type:   &openapi3.Types{"string"},
							Format: "date-time",
						},
					},
				},
			},
			wantValid: true,
		},
		{
			name: "Invalid schema - missing type",
			schema: &openapi3.Schema{
				Properties: map[string]*openapi3.SchemaRef{
					"event_type": {
						Value: &openapi3.Schema{
							Enum: []interface{}{"created", "updated", "deleted"},
						},
					},
				},
			},
			wantValid:     true,
			wantErrorLen:  1,
			wantErrorDesc: "(root): type is required",
		},
		{
			name: "Invalid schema - invalid type",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"invalid_type"},
			},
			wantValid:     false,
			wantErrorLen:  2,
			wantErrorDesc: "Must validate at least one schema (anyOf)",
		},
		{
			name: "Invalid schema - invalid format",
			schema: &openapi3.Schema{
				Type:   &openapi3.Types{"string"},
				Format: "invalid_format",
			},
			wantValid:     true,
			wantErrorLen:  1,
			wantErrorDesc: "(root).format: format must be one of the following: \"date\", \"date-time\", \"duration\", \"email\", \"hostname\", \"idn-email\", \"idn-hostname\", \"ipv4\", \"ipv6\", \"iri\", \"iri-reference\", \"json-pointer\", \"regex\", \"relative-json-pointer\", \"time\", \"uri\", \"uri-reference\", \"uri-template\", \"uuid\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &Webhook{Schema: tt.schema}
			result, err := webhook.ValidateSchema()
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantValid, result.IsValid)

			if !tt.wantValid {
				require.Equal(t, tt.wantErrorLen, len(result.Errors))
				if tt.wantErrorDesc != "" {
					require.Equal(t, tt.wantErrorDesc, result.Errors[0].Description)
				}
			}
		})
	}
}

func TestWebhook_ValidateData(t *testing.T) {
	webhook := &Webhook{
		Name:        "test",
		Description: "Test webhook",
		Schema: &openapi3.Schema{
			Type:     &openapi3.Types{"object"},
			Required: []string{"event_type", "timestamp"},
			Properties: map[string]*openapi3.SchemaRef{
				"event_type": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
						Enum: []interface{}{"created", "updated", "deleted"},
					},
				},
				"timestamp": {
					Value: &openapi3.Schema{
						Type:   &openapi3.Types{"string"},
						Format: "date-time",
					},
				},
				"data": {
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"id": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
							"value": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"number"},
								},
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		data          interface{}
		wantValid     bool
		wantErrorLen  int
		wantErrorDesc string
	}{
		{
			name: "Valid data",
			data: map[string]interface{}{
				"event_type": "created",
				"timestamp":  "2024-03-20T10:00:00Z",
				"data": map[string]interface{}{
					"id":    "123",
					"value": 42.5,
				},
			},
			wantValid: true,
		},
		{
			name: "Missing required field",
			data: map[string]interface{}{
				"event_type": "created",
			},
			wantValid:     false,
			wantErrorLen:  1,
			wantErrorDesc: "timestamp is required",
		},
		{
			name: "Invalid enum value",
			data: map[string]interface{}{
				"event_type": "invalid",
				"timestamp":  "2024-03-20T10:00:00Z",
			},
			wantValid:     false,
			wantErrorLen:  1,
			wantErrorDesc: "event_type must be one of the following: \"created\", \"updated\", \"deleted\"",
		},
		{
			name: "Invalid timestamp format",
			data: map[string]interface{}{
				"event_type": "created",
				"timestamp":  "invalid",
			},
			wantValid:     false,
			wantErrorLen:  1,
			wantErrorDesc: "Does not match format 'date-time'",
		},
		{
			name: "Invalid data types",
			data: map[string]interface{}{
				"event_type": "created",
				"timestamp":  "2024-03-20T10:00:00Z",
				"data": map[string]interface{}{
					"id":    123,            // Should be string
					"value": "not a number", // Should be number
				},
			},
			wantValid:    false,
			wantErrorLen: 2,
		},
		{
			name:      "Valid JSON string input",
			data:      `{"event_type": "created", "timestamp": "2024-03-20T10:00:00Z"}`,
			wantValid: true,
		},
		{
			name:      "Valid JSON bytes input",
			data:      []byte(`{"event_type": "created", "timestamp": "2024-03-20T10:00:00Z"}`),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := webhook.ValidateData(tt.data)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantValid, result.IsValid)

			if !tt.wantValid {
				require.Equal(t, tt.wantErrorLen, len(result.Errors))
				if tt.wantErrorDesc != "" {
					require.Equal(t, tt.wantErrorDesc, result.Errors[0].Description)
				}
			}
		})
	}
}

func TestWebhook_ValidateData_InvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		schema     *openapi3.Schema
		data       interface{}
		wantErrMsg string
	}{
		{
			name: "Invalid JSON string",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
			},
			data:       "invalid json",
			wantErrMsg: "data validation failed: invalid character 'i' looking for beginning of value",
		},
		{
			name: "JSON string should not match",
			schema: &openapi3.Schema{
				Extensions: map[string]interface{}{
					"foo": "bar",
				},
			},
			data:       "invalid json",
			wantErrMsg: "data validation failed: invalid character 'i' looking for beginning of value",
		},
		{
			name:       "Nil schema",
			schema:     nil,
			data:       map[string]interface{}{},
			wantErrMsg: "schema validation failed: schema is required",
		},
		{
			name: "Invalid schema",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"invalid_type"},
			},
			data:       map[string]interface{}{},
			wantErrMsg: "schema validation failed: (root).type: type must be one of the following: \"array\", \"boolean\", \"integer\", \"null\", \"number\", \"object\", \"string\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &Webhook{Schema: tt.schema}
			result, err := webhook.ValidateData(tt.data)
			require.Error(t, err)
			require.Nil(t, result)
			require.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}
