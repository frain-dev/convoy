package services

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateFanoutEventService struct {
	EndpointRepo   datastore.EndpointRepository
	EventRepo      datastore.EventRepository
	PortalLinkRepo datastore.PortalLinkRepository
	Queue          queue.Queuer

	NewMessage *models.FanoutEvent
	Project    *datastore.Project
}

func (e *CreateFanoutEventService) Run(ctx context.Context) (*datastore.Event, error) {
	if e.Project == nil {
		return nil, &ServiceError{ErrMsg: "an error occurred while creating event - invalid project"}
	}

	if err := util.Validate(e.NewMessage); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	var isDuplicate bool
	if !util.IsStringEmpty(e.NewMessage.IdempotencyKey) {
		events, err := e.EventRepo.FindEventsByIdempotencyKey(ctx, e.Project.UID, e.NewMessage.IdempotencyKey)
		if err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		isDuplicate = len(events) > 0
	}

	endpoints, err := e.EndpointRepo.FindEndpointsByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(endpoints) == 0 {
		_, err := e.PortalLinkRepo.FindPortalLinkByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
		if err != nil {
			if errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &ServiceError{ErrMsg: ErrNoValidOwnerIDEndpointFound.Error()}
			}

			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	ev := &newEvent{
		Data:           e.NewMessage.Data,
		EventType:      e.NewMessage.EventType,
		IdempotencyKey: e.NewMessage.IdempotencyKey,
		Raw:            string(e.NewMessage.Data),
		CustomHeaders:  e.NewMessage.CustomHeaders,
		IsDuplicate:    isDuplicate,
	}

	event, err := createEvent(ctx, endpoints, ev, e.Project, e.Queue)
	if err != nil {
		return nil, err
	}

	return event, nil
}
