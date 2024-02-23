package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

type UpdateCatalogueService struct {
	CatalogueRepo   datastore.EventCatalogueRepository
	UpdateCatalogue models.UpdateCatalogue
	Project         *datastore.Project
}

func (c *UpdateCatalogueService) Run(ctx context.Context) (*datastore.EventCatalogue, error) {
	catalogue, err := c.CatalogueRepo.FindEventCatalogueByProjectID(ctx, c.Project.UID)
	if err != nil && !errors.Is(err, datastore.ErrCatalogueNotFound) {
		return nil, &ServiceError{ErrMsg: "unable to fetch catalogue", Err: err}
	}

	switch catalogue.Type {
	case datastore.EventsDataCatalogueType:
		catalogue.Events = c.UpdateCatalogue.Events
	case datastore.OpenAPICatalogueType:
		catalogue.OpenAPISpec = c.UpdateCatalogue.OpenAPISpec
	}

	err = c.CatalogueRepo.UpdateEventCatalogue(ctx, catalogue)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to update catalogue", Err: err}
	}
	return catalogue, nil
}
