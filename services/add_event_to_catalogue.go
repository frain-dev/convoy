package services

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
)

type AddEventToCatalogueService struct {
	CatalogueRepo  datastore.EventCatalogueRepository
	EventRepo      datastore.EventRepository
	CatalogueEvent models.AddEventToCatalogue
	Project        *datastore.Project
}

func (c *AddEventToCatalogueService) Run(ctx context.Context) (*datastore.EventCatalogue, error) {
	catalogue, err := c.CatalogueRepo.FindEventCatalogueByProjectID(ctx, c.Project.UID)
	if err != nil && !errors.Is(err, datastore.ErrCatalogueNotFound) {
		return nil, &ServiceError{ErrMsg: "unable to fetch event catalogue", Err: err}
	}

	event, err := c.EventRepo.FindEventByID(ctx, c.Project.UID, c.CatalogueEvent.EventID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to fetch event", Err: err}
	}

	switch {
	case err == nil:
		if catalogue.Type != datastore.EventsDataCatalogueType {
			return nil, &ServiceError{ErrMsg: "you cannot add event to an openapi catalogue", Err: err}
		}

		catalogue.Events = append(catalogue.Events, datastore.EventDataCatalogue{
			Name:    c.CatalogueEvent.Name,
			EventID: event.UID,
			Data:    event.Data,
		})

		err = c.CatalogueRepo.UpdateEventCatalogue(ctx, catalogue)
		if err != nil {
			return nil, &ServiceError{ErrMsg: "unable to update event catalogue", Err: err}
		}

	case errors.Is(err, datastore.ErrCatalogueNotFound):
		catalogue = &datastore.EventCatalogue{
			UID:       ulid.Make().String(),
			ProjectID: c.Project.UID,
			Type:      datastore.EventsDataCatalogueType,
			Events: datastore.EventDataCatalogues{
				{
					Name:    c.CatalogueEvent.Name,
					EventID: event.UID,
					Data:    event.Data,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err = c.CatalogueRepo.CreateEventCatalogue(ctx, catalogue)
		if err != nil {
			return nil, &ServiceError{ErrMsg: "unable to create event catalogue", Err: err}
		}
	}

	return catalogue, nil
}
