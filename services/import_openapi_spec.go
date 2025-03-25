package services

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/openapi"
	"github.com/oklog/ulid/v2"
	"time"
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

func (im *ImportOpenapiSpecService) Run(ctx context.Context) ([]datastore.ProjectEventType, error) {
	var types []datastore.ProjectEventType

	webhooks, err := im.Converter.ExtractWebhooks()
	if err != nil {
		return nil, err
	}

	for eventType, schema := range webhooks.Webhooks {
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
			ev.JSONSchema = schema.AsBytes()
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
				JSONSchema:  schema.AsBytes(),
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
