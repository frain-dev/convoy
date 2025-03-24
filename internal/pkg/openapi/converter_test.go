package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestConverter_ExtractWebhooks(t *testing.T) {
	tests := []struct {
		name           string
		specFile       string
		expectedCount  int
		expectedNames  []string
		expectedFields map[string][]string
	}{
		{
			name:          "OpenAPI 3.0 with x-webhooks",
			specFile:      "testdata/test-3.0.yml",
			expectedCount: 2,
			expectedNames: []string{"barberSaasWebhook", "electricalEquipmentWebhook"},
			expectedFields: map[string][]string{
				"barberSaasWebhook": {
					"barberId", "barberName", "booking", "schedule", "services",
				},
				"electricalEquipmentWebhook": {
					"id", "name", "brand", "category", "price", "currency", "inStock", "availableVariants",
				},
			},
		},
		{
			name:          "OpenAPI 3.1 with webhooks",
			specFile:      "testdata/test-3.1.yml",
			expectedCount: 2,
			expectedNames: []string{"barberSaasWebhook", "electricalEquipmentWebhook"},
			expectedFields: map[string][]string{
				"barberSaasWebhook": {
					"barberId", "barberName", "booking", "schedule", "services",
				},
				"electricalEquipmentWebhook": {
					"id", "name", "brand", "category", "price", "currency", "inStock", "availableVariants",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read and parse the OpenAPI spec
			specBytes, err := os.ReadFile(tt.specFile)
			require.NoError(t, err)

			// Load as OpenAPI 3.x
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true
			swagger, err := loader.LoadFromData(specBytes)
			require.NoError(t, err)

			// Create converter
			conv, err := New(swagger)
			require.NoError(t, err)

			// Extract webhooks
			collection, err := conv.ExtractWebhooks()
			require.NoError(t, err)

			// Verify webhook count
			require.Equal(t, tt.expectedCount, len(collection.Webhooks))

			// Verify webhook names and schemas
			for name, expectedFields := range tt.expectedFields {
				schema, ok := collection.Webhooks[name]
				require.True(t, ok, "webhook %s not found", name)
				require.NotNil(t, schema, "schema for webhook %s is nil", name)

				// Verify required fields
				require.ElementsMatch(t, expectedFields, schema.Required)

				// Verify example is present
				require.NotNil(t, schema.Example, "example should not be empty for webhook %s", name)
			}

			// Write the output to a file for manual inspection if needed
			outFile := filepath.Join("testdata", "out-"+filepath.Base(tt.specFile)+".json")
			outBytes, err := json.MarshalIndent(collection, "", "  ")
			require.NoError(t, err)
			err = os.WriteFile(outFile, outBytes, 0644)
			require.NoError(t, err)

			// Register cleanup to delete the output file after the test completes
			t.Cleanup(func() {
				_ = os.Remove(outFile)
			})
		})
	}
}

func TestConverter_ExtractWebhooks_Examples(t *testing.T) {
	tests := []struct {
		name              string
		specFile          string
		webhook           string
		expectedExamples  map[string]interface{}
		expectedPropTypes map[string]string
	}{
		{
			name:     "OpenAPI 3.0 BarberSaaS webhook examples",
			specFile: "testdata/test-3.0.yml",
			webhook:  "barberSaasWebhook",
			expectedExamples: map[string]interface{}{
				"barberId":   "barber_123",
				"barberName": "John Smith",
				"booking": map[string]interface{}{
					"bookingId":     "book_456",
					"customerEmail": "alice@example.com",
					"customerName":  "Alice Johnson",
					"serviceId":     "service_789",
					"date":          "2024-03-25",
					"startTime":     "14:30:00",
					"endTime":       "15:30:00",
				},
			},
			expectedPropTypes: map[string]string{
				"barberId":   "string",
				"barberName": "string",
				"booking":    "object",
			},
		},
		{
			name:     "OpenAPI 3.1 ElectricalEquipment webhook examples",
			specFile: "testdata/test-3.1.yml",
			webhook:  "electricalEquipmentWebhook",
			expectedExamples: map[string]interface{}{
				"id":          "prod_123",
				"name":        "Professional Hair Dryer X2000",
				"brand":       "StylePro",
				"category":    "Hair Care Equipment",
				"price":       199.99,
				"currency":    "USD",
				"inStock":     true,
				"description": "Professional-grade hair dryer with ionic technology",
			},
			expectedPropTypes: map[string]string{
				"id":          "string",
				"name":        "string",
				"brand":       "string",
				"category":    "string",
				"price":       "number",
				"currency":    "string",
				"inStock":     "boolean",
				"description": "string",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read and parse the OpenAPI spec
			specBytes, err := os.ReadFile(tt.specFile)
			require.NoError(t, err)

			// Load as OpenAPI 3.x
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true
			swagger, err := loader.LoadFromData(specBytes)
			require.NoError(t, err)

			// Create converter
			conv, err := New(swagger)
			require.NoError(t, err)

			// Extract webhooks
			collection, err := conv.ExtractWebhooks()
			require.NoError(t, err)

			// Get the specific webhook
			schema, ok := collection.Webhooks[tt.webhook]
			require.True(t, ok, "webhook %s not found", tt.webhook)
			require.NotNil(t, schema, "schema for webhook %s is nil", tt.webhook)

			// Verify property types
			for prop, expectedType := range tt.expectedPropTypes {
				propSchema, ok := schema.Properties[prop]
				require.True(t, ok, "property %s not found", prop)
				require.Equal(t, expectedType, string((*propSchema.Value.Type)[0]), "property %s type mismatch", prop)
			}

			// Verify example values
			example := schema.Example.(map[string]interface{})
			for field, expectedValue := range tt.expectedExamples {
				actualValue, ok := example[field]
				require.True(t, ok, "example field %s not found", field)
				require.Equal(t, expectedValue, actualValue, "example field %s value mismatch", field)
			}

			// Write the output to a file for manual inspection if needed
			outFile := filepath.Join("testdata", "out-"+filepath.Base(tt.specFile)+".json")
			outBytes, err := json.MarshalIndent(collection, "", "  ")
			require.NoError(t, err)
			err = os.WriteFile(outFile, outBytes, 0644)
			require.NoError(t, err)

			// Register cleanup to delete the output file after the test completes
			t.Cleanup(func() {
				_ = os.Remove(outFile)
			})
		})
	}
}
