package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConverter_ExtractWebhooks(t *testing.T) {
	tests := []struct {
		name                      string
		specFile                  string
		expectedCount             int
		expectedTypes             map[string]string
		expectedFields            map[string][]string
		expectedDescriptions      map[string]string
		expectedFieldDescriptions map[string]map[string]string
	}{
		{
			name:          "OpenAPI 3.0",
			specFile:      "testdata/test-3.0.yml",
			expectedCount: 2,
			expectedTypes: map[string]string{
				"event_type":     "string",
				"appointment_id": "string",
				"customer_name":  "string",
				"service_type":   "string",
				"timestamp":      "string",
				"notes":          "string",
			},
			expectedFields: map[string][]string{
				"barber": {"event_type", "appointment_id", "customer_name", "service_type", "timestamp"},
			},
			expectedDescriptions: map[string]string{
				"barber":     "Webhook for barber shop events",
				"electrical": "Webhook for electrical equipment inventory updates",
			},
			expectedFieldDescriptions: map[string]map[string]string{
				"barber": {
					"event_type":     "Type of event that occurred (e.g., appointment created, updated, cancelled)",
					"appointment_id": "Unique identifier for the appointment",
					"customer_name":  "Full name of the customer",
					"service_type":   "Type of service booked (e.g., Haircut, Shave)",
					"timestamp":      "Date and time when the event occurred",
					"notes":          "Additional notes or comments about the appointment",
				},
				"electrical": {
					"event_type": "Type of inventory event (e.g., restock, sold, damaged)",
					"item_id":    "Unique identifier for the electrical equipment",
					"quantity":   "Number of items affected by the event",
					"location":   "Storage location or warehouse identifier",
					"timestamp":  "Date and time when the inventory event occurred",
					"notes":      "Additional notes about the inventory update",
				},
			},
		},
		{
			name:          "OpenAPI 3.1",
			specFile:      "testdata/test-3.1.yml",
			expectedCount: 2,
			expectedTypes: map[string]string{
				"event_type":     "string",
				"appointment_id": "string",
				"customer_name":  "string",
				"service_type":   "string",
				"timestamp":      "string",
				"notes":          "string",
			},
			expectedFields: map[string][]string{
				"barber": {"event_type", "appointment_id", "customer_name", "service_type", "timestamp"},
			},
			expectedDescriptions: map[string]string{
				"barber":     "Webhook for barber shop events",
				"electrical": "Webhook for electrical equipment inventory updates",
			},
			expectedFieldDescriptions: map[string]map[string]string{
				"barber": {
					"event_type":     "Type of event that occurred (e.g., appointment created, updated, cancelled)",
					"appointment_id": "Unique identifier for the appointment",
					"customer_name":  "Full name of the customer",
					"service_type":   "Type of service booked (e.g., Haircut, Shave)",
					"timestamp":      "Date and time when the event occurred",
					"notes":          "Additional notes or comments about the appointment",
				},
				"electrical": {
					"event_type": "Type of inventory event (e.g., restock, sold, damaged)",
					"item_id":    "Unique identifier for the electrical equipment",
					"quantity":   "Number of items affected by the event",
					"location":   "Storage location or warehouse identifier",
					"timestamp":  "Date and time when the inventory event occurred",
					"notes":      "Additional notes about the inventory update",
				},
			},
		},
		{
			name:          "OpenAPI 2.0",
			specFile:      "testdata/test-2.0.yml",
			expectedCount: 2,
			expectedTypes: map[string]string{
				"event_type":     "string",
				"appointment_id": "string",
				"customer_name":  "string",
				"service_type":   "string",
				"timestamp":      "string",
				"notes":          "string",
			},
			expectedFields: map[string][]string{
				"barber": {"event_type", "appointment_id", "customer_name", "service_type", "timestamp"},
			},
			expectedDescriptions: map[string]string{
				"barber":     "Webhook for barber shop events",
				"electrical": "Webhook for electrical equipment inventory updates",
			},
			expectedFieldDescriptions: map[string]map[string]string{
				"barber": {
					"event_type":     "Type of event that occurred (e.g., appointment created, updated, cancelled)",
					"appointment_id": "Unique identifier for the appointment",
					"customer_name":  "Full name of the customer",
					"service_type":   "Type of service booked (e.g., Haircut, Shave)",
					"timestamp":      "Date and time when the event occurred",
					"notes":          "Additional notes or comments about the appointment",
				},
				"electrical": {
					"event_type": "Type of inventory event (e.g., restock, sold, damaged)",
					"item_id":    "Unique identifier for the electrical equipment",
					"quantity":   "Number of items affected by the event",
					"location":   "Storage location or warehouse identifier",
					"timestamp":  "Date and time when the inventory event occurred",
					"notes":      "Additional notes about the inventory update",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read and parse the OpenAPI spec
			data, err := os.ReadFile(tt.specFile)
			require.NoError(t, err)

			var doc interface{}
			if tt.name == "OpenAPI 2.0" {
				// Parse YAML into a map first
				var rawDoc map[string]interface{}
				err = yaml.Unmarshal(data, &rawDoc)
				require.NoError(t, err)

				// Convert map to OpenAPI 2.0 document
				var v2doc openapi2.T
				v2bytes, err := json.Marshal(rawDoc)
				require.NoError(t, err)
				err = json.Unmarshal(v2bytes, &v2doc)
				require.NoError(t, err)
				doc = &v2doc
			} else {
				// For OpenAPI 3.0 and 3.1, use the loader
				loader := openapi3.NewLoader()
				loader.IsExternalRefsAllowed = true
				v3doc, err := loader.LoadFromData(data)
				require.NoError(t, err)

				// For OpenAPI 3.0, webhooks are in x-webhooks extension
				switch tt.name {
				case "OpenAPI 3.0":
					if v3doc.Extensions == nil {
						v3doc.Extensions = make(map[string]interface{})
					}
					var rawDoc map[string]interface{}
					err = yaml.Unmarshal(data, &rawDoc)
					require.NoError(t, err)
					if webhooks, ok := rawDoc["x-webhooks"]; ok {
						t.Logf("Found x-webhooks in OpenAPI 3.0: %+v", webhooks)
						v3doc.Extensions["x-webhooks"] = webhooks
					} else {
						t.Logf("No x-webhooks found in OpenAPI 3.0")
					}
				case "OpenAPI 3.1":
					if webhooks, ok := v3doc.Extensions["webhooks"]; ok {
						t.Logf("Found webhooks in OpenAPI 3.1: %+v", webhooks)
					} else {
						t.Logf("No webhooks found in OpenAPI 3.1")
					}
				}

				doc = v3doc
			}

			// Create converter and extract webhooks
			conv, err := New(doc)
			require.NoError(t, err)

			collection, err := conv.ExtractWebhooks()
			require.NoError(t, err)

			// Verify number of webhooks
			require.Equal(t, tt.expectedCount, len(collection.Webhooks))

			// Verify webhook schema properties and types
			for name, webhook := range collection.Webhooks {
				if fields, ok := tt.expectedFields[name]; ok {
					require.NotNil(t, webhook.Schema)
					require.NotNil(t, webhook.Schema.Required)
					require.ElementsMatch(t, fields, webhook.Schema.Required)

					for prop, propSchema := range webhook.Schema.Properties {
						if expectedType, ok := tt.expectedTypes[prop]; ok {
							require.Equal(t, expectedType, string((*propSchema.Value.Type)[0]), "property %s type mismatch", prop)
						}
					}
				}

				// Verify webhook description
				if expectedDesc, ok := tt.expectedDescriptions[name]; ok {
					require.Equal(t, expectedDesc, webhook.Description, "webhook %s description mismatch", name)
				}

				for propName, prop := range webhook.Schema.Properties {
					if expectedFieldDesc, ok := tt.expectedFieldDescriptions[name][propName]; ok {
						require.Equal(t, expectedFieldDesc, prop.Value.Description, "field %s description mismatch", propName)
					}
				}
			}

			// Write output to file for manual inspection
			outFile := filepath.Join("testdata", "out-"+filepath.Base(tt.specFile)+".json")
			out, err := json.MarshalIndent(collection, "", "  ")
			require.NoError(t, err)
			err = os.WriteFile(outFile, out, 0644)
			require.NoError(t, err)

			// Register cleanup to delete the output file after test completes
			t.Cleanup(func() {
				_ = os.Remove(outFile)
			})
		})
	}
}

func TestConverter_ExtractWebhooks_Examples(t *testing.T) {
	tests := []struct {
		name            string
		specFile        string
		webhookName     string
		expectedCount   int
		expectedExample map[string]interface{}
	}{
		{
			name:          "OpenAPI 3.0 - BarberSaaS",
			specFile:      "testdata/test-3.0.yml",
			webhookName:   "barber",
			expectedCount: 2,
			expectedExample: map[string]interface{}{
				"event_type":     "appointment_created",
				"appointment_id": "123e4567-e89b-12d3-a456-426614174000",
				"customer_name":  "John Doe",
				"service_type":   "Haircut",
				"timestamp":      "2024-03-20T10:00:00Z",
				"notes":          "First time customer",
			},
		},
		{
			name:          "OpenAPI 3.1 - ElectricalEquipment",
			specFile:      "testdata/test-3.1.yml",
			webhookName:   "electrical",
			expectedCount: 2,
			expectedExample: map[string]interface{}{
				"event_type": "item_restocked",
				"item_id":    "789e0123-f45b-67d8-a456-426614174000",
				"quantity":   float64(50),
				"location":   "Warehouse-A",
				"timestamp":  "2024-03-20T14:30:00Z",
				"notes":      "Bulk order received",
			},
		},
		{
			name:          "OpenAPI 2.0 - BarberSaaS",
			specFile:      "testdata/test-2.0.yml",
			webhookName:   "barber",
			expectedCount: 2,
			expectedExample: map[string]interface{}{
				"event_type":     "appointment_created",
				"appointment_id": "123e4567-e89b-12d3-a456-426614174000",
				"customer_name":  "John Doe",
				"service_type":   "Haircut",
				"timestamp":      "2024-03-20T10:00:00Z",
				"notes":          "First time customer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read and parse the OpenAPI spec
			data, err := os.ReadFile(tt.specFile)
			require.NoError(t, err)

			var doc interface{}
			if tt.name == "OpenAPI 2.0 - BarberSaaS" {
				// Parse YAML into a map first
				var rawDoc map[string]interface{}
				err = yaml.Unmarshal(data, &rawDoc)
				require.NoError(t, err)

				// Convert map to OpenAPI 2.0 document
				var v2doc openapi2.T
				v2bytes, err := json.Marshal(rawDoc)
				require.NoError(t, err)
				err = json.Unmarshal(v2bytes, &v2doc)
				require.NoError(t, err)
				doc = &v2doc
			} else {
				// For OpenAPI 3.0 and 3.1, use the loader
				loader := openapi3.NewLoader()
				loader.IsExternalRefsAllowed = true
				v3doc, err := loader.LoadFromData(data)
				require.NoError(t, err)

				// For OpenAPI 3.0, webhooks are in x-webhooks extension
				if strings.Contains(tt.name, "OpenAPI 3.0") {
					if v3doc.Extensions == nil {
						v3doc.Extensions = make(map[string]interface{})
					}
					var rawDoc map[string]interface{}
					err = yaml.Unmarshal(data, &rawDoc)
					require.NoError(t, err)
					if webhooks, ok := rawDoc["x-webhooks"]; ok {
						t.Logf("Found x-webhooks in OpenAPI 3.0: %+v", webhooks)
						v3doc.Extensions["x-webhooks"] = webhooks
					} else {
						t.Logf("No x-webhooks found in OpenAPI 3.0")
					}
				} else if strings.Contains(tt.name, "OpenAPI 3.1") {
					if webhooks, ok := v3doc.Extensions["webhooks"]; ok {
						t.Logf("Found webhooks in OpenAPI 3.1: %+v", webhooks)
					} else {
						t.Logf("No webhooks found in OpenAPI 3.1")
					}
				}

				doc = v3doc
			}

			// Create converter and extract webhooks
			conv, err := New(doc)
			require.NoError(t, err)

			collection, err := conv.ExtractWebhooks()
			require.NoError(t, err)

			// Verify webhook schema and example
			webhook, ok := collection.Webhooks[tt.webhookName]
			require.True(t, ok, "webhook %s not found", tt.webhookName)
			require.NotNil(t, webhook)
			require.NotNil(t, webhook.Schema)
			require.Equal(t, tt.expectedExample, webhook.Schema.Example)
		})
	}
}
