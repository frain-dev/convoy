package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestWebhook_Validate(t *testing.T) {
	// Create a test webhook with a schema
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
			name: "Invalid data type",
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
			name:      "String input",
			data:      `{"event_type": "created", "timestamp": "2024-03-20T10:00:00Z"}`,
			wantValid: true,
		},
		{
			name:      "Bytes input",
			data:      []byte(`{"event_type": "created", "timestamp": "2024-03-20T10:00:00Z"}`),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := webhook.Validate(tt.data)
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

func TestWebhook_Validate_InvalidInput(t *testing.T) {
	webhook := &Webhook{
		Schema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
		},
	}

	// Test with invalid JSON string
	result, err := webhook.Validate("invalid json")
	require.Error(t, err)
	require.Nil(t, result)

	// Test with nil schema
	webhook.Schema = nil
	result, err = webhook.Validate(map[string]interface{}{})
	require.Error(t, err)
	require.Nil(t, result)
}
