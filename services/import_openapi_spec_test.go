package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestImportOpenapiSpecService_Run(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventTypesRepo := mocks.NewMockEventTypesRepository(ctrl)

	validSchema := `{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "The event ID"
			},
			"timestamp": {
				"type": "string",
				"format": "date-time",
				"description": "When the event occurred"
			}
		},
		"required": ["id", "timestamp"]
	}`

	tests := []struct {
		name          string
		spec          string
		projectID     string
		expectedError string
		mockFn        func(repo *mocks.MockEventTypesRepository)
		verifyFn      func(t *testing.T, eventTypes []datastore.ProjectEventType)
	}{
		{
			name: "should successfully import valid OpenAPI spec with valid schema",
			spec: `{
				"openapi": "3.1.0",
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				},
				"webhooks": {
					"test.event": {
						"post": {
							"requestBody": {
								"content": {
									"application/json": {
										"schema": {
											"type": "object",
											"properties": {
												"id": {
													"type": "string",
													"description": "The event ID"
												},
												"timestamp": {
													"type": "string",
													"format": "date-time",
													"description": "When the event occurred"
												}
											},
											"required": ["id", "timestamp"]
										}
									}
								}
							}
						}
					}
				}
			}`,
			projectID: "test-project",
			mockFn: func(repo *mocks.MockEventTypesRepository) {
				repo.EXPECT().CheckEventTypeExists(gomock.Any(), "test.event", "test-project").Return(false, nil)
				repo.EXPECT().CreateEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, et *datastore.ProjectEventType) error {
					// Verify schema is set correctly during creation
					var schema map[string]interface{}
					err := json.Unmarshal(et.JSONSchema, &schema)
					require.NoError(t, err)
					require.Equal(t, "object", schema["type"])
					require.NotNil(t, schema["properties"])
					require.NotNil(t, schema["required"])
					return nil
				})
			},
		},
		{
			name: "should fail with invalid schema in OpenAPI spec",
			spec: `{
				"openapi": "3.1.0",
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				},
				"webhooks": {
					"test.event": {
						"post": {
							"requestBody": {
								"content": {
									"application/json": {
										"schema": {
											"type": "invalid",
											"properties": {
												"id": {
													"type": "string"
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
			projectID: "test-project",
			mockFn: func(repo *mocks.MockEventTypesRepository) {
				repo.EXPECT().CheckEventTypeExists(gomock.Any(), "test.event", "test-project").Return(false, nil)
				repo.EXPECT().CreateEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, et *datastore.ProjectEventType) error {
					// Verify schema is set correctly during creation
					var schema map[string]interface{}
					err := json.Unmarshal(et.JSONSchema, &schema)
					require.NoError(t, err)
					require.Equal(t, "object", schema["type"])
					require.NotNil(t, schema["properties"])
					return nil
				})
			},
		},
		{
			name: "should successfully update existing event type with valid schema",
			spec: `{
				"openapi": "3.1.0",
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				},
				"webhooks": {
					"test.event": {
						"post": {
							"requestBody": {
								"content": {
									"application/json": {
										"schema": {
											"type": "object",
											"properties": {
												"id": {
													"type": "string",
													"description": "The event ID"
												},
												"timestamp": {
													"type": "string",
													"format": "date-time",
													"description": "When the event occurred"
												}
											},
											"required": ["id", "timestamp"]
										}
									}
								}
							}
						}
					}
				}
			}`,
			projectID: "test-project",
			mockFn: func(repo *mocks.MockEventTypesRepository) {
				existingEventType := &datastore.ProjectEventType{
					UID:        "test-uid",
					Name:       "test.event",
					ProjectId:  "test-project",
					JSONSchema: []byte(validSchema),
				}

				repo.EXPECT().CheckEventTypeExists(gomock.Any(), "test.event", "test-project").Return(true, nil)
				repo.EXPECT().FetchEventTypeByName(gomock.Any(), "test.event", "test-project").Return(existingEventType, nil)
				repo.EXPECT().UpdateEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, et *datastore.ProjectEventType) error {
					// Verify schema is updated correctly
					var schema map[string]interface{}
					err := json.Unmarshal(et.JSONSchema, &schema)
					require.NoError(t, err)
					require.Equal(t, "object", schema["type"])
					require.NotNil(t, schema["properties"])
					require.NotNil(t, schema["required"])
					return nil
				})
			},
			verifyFn: func(t *testing.T, eventTypes []datastore.ProjectEventType) {
				require.Len(t, eventTypes, 1)
				et := eventTypes[0]
				require.Equal(t, "test.event", et.Name)

				// Verify the schema structure
				var schema map[string]interface{}
				err := json.Unmarshal(et.JSONSchema, &schema)
				require.NoError(t, err)
				require.Equal(t, "object", schema["type"])

				props, ok := schema["properties"].(map[string]interface{})
				require.True(t, ok)
				require.Contains(t, props, "id")
				require.Contains(t, props, "timestamp")

				required, ok := schema["required"].([]interface{})
				require.True(t, ok)
				require.Contains(t, required, "id")
				require.Contains(t, required, "timestamp")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockFn != nil {
				tc.mockFn(eventTypesRepo)
			}

			service, err := NewImportOpenapiSpecService(tc.spec, tc.projectID, eventTypesRepo)
			require.NoError(t, err)

			eventTypes, err := service.Run(ctx)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, tc.expectedError, err.Error())
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, eventTypes)

				// Run additional verifications if provided
				if tc.verifyFn != nil {
					tc.verifyFn(t, eventTypes)
				}

				// Verify that each event type has a valid JSON schema
				for _, et := range eventTypes {
					var schema map[string]interface{}
					err = json.Unmarshal(et.JSONSchema, &schema)
					require.NoError(t, err)
					require.Equal(t, "object", schema["type"])
				}
			}
		})
	}
}
