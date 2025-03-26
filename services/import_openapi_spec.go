package services

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oklog/ulid/v2"
)

type ImportOpenapiSpecService struct {
	Converter      *openapi.Converter
	EventTypesRepo datastore.EventTypesRepository
	ProjectId      string
}

func NewImportOpenapiSpecService(data, projectId string, eventTypesRepo datastore.EventTypesRepository) (*ImportOpenapiSpecService, error) {
	converter, err := openapi.NewFromBytes([]byte(data))
	if err != nil {
		return nil, err
	}

	return &ImportOpenapiSpecService{
		EventTypesRepo: eventTypesRepo,
		Converter:      converter,
		ProjectId:      projectId,
	}, nil
}

func (im *ImportOpenapiSpecService) validateSchema(schema []byte) error {
	webhook := &openapi.Webhook{
		Schema: &openapi3.Schema{},
	}

	// Unmarshal the schema into the webhook's Schema field
	if err := webhook.Schema.UnmarshalJSON(schema); err != nil {
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

func (im *ImportOpenapiSpecService) Run(ctx context.Context) ([]datastore.ProjectEventType, error) {
	var types []datastore.ProjectEventType

	webhooks, err := im.Converter.ExtractWebhooks()
	if err != nil {
		return nil, err
	}

	for eventType, schema := range webhooks.Webhooks {
		// Validate the schema before proceeding
		schemaBytes := schema.AsBytes()
		if err := im.validateSchema(schemaBytes); err != nil {
			return nil, fmt.Errorf("invalid schema for event type %s: %v", eventType, err)
		}

		exists, existsErr := im.EventTypesRepo.CheckEventTypeExists(ctx, eventType, im.ProjectId)
		if existsErr != nil {
			return nil, existsErr
		}

		if exists {
			ev, innerErr := im.EventTypesRepo.FetchEventTypeByName(ctx, eventType, im.ProjectId)
			if innerErr != nil {
				return nil, innerErr
			}

			// update the event type
			ev.JSONSchema = schemaBytes
			ev.Description = schema.Description

			updateErr := im.EventTypesRepo.UpdateEventType(ctx, ev)
			if updateErr != nil {
				return nil, updateErr
			}

			types = append(types, *ev)
		} else {
			// create the event type
			newEventType := &datastore.ProjectEventType{
				UID:         ulid.Make().String(),
				Name:        eventType,
				ProjectId:   im.ProjectId,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				JSONSchema:  schemaBytes,
				Description: schema.Description,
			}

			createErr := im.EventTypesRepo.CreateEventType(ctx, newEventType)
			if createErr != nil {
				return nil, createErr
			}

			types = append(types, *newEventType)
		}
	}

	return types, nil
}
