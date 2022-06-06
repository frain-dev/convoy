package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/dchest/uniuri"
)

type SourceService struct {
	sourceRepo datastore.SourceRepository
}

func NewSourceService(sourceRepo datastore.SourceRepository) *SourceService {
	return &SourceService{sourceRepo: sourceRepo}
}

func (s *SourceService) CreateSource(ctx context.Context, newSource *models.Source, g *datastore.Group) (*datastore.Source, error) {
	if err := util.Validate(newSource); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	source := &datastore.Source{
		UID:            uuid.New().String(),
		GroupID:        g.UID,
		MaskID:         uniuri.NewLen(16),
		Name:           newSource.Name,
		Type:           newSource.Type,
		Verifier:       &newSource.Verifier,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := s.sourceRepo.CreateSource(ctx, source)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create source"))
	}

	return source, nil
}

func (s *SourceService) UpdateSource(ctx context.Context, g *datastore.Group, sourceUpdate *models.UpdateSource, source *datastore.Source) (*datastore.Source, error) {
	if err := util.Validate(sourceUpdate); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	source.Name = *sourceUpdate.Name
	source.Verifier = &sourceUpdate.Verifier
	source.Type = sourceUpdate.Type

	if sourceUpdate.IsDisabled != nil {
		source.IsDisabled = *sourceUpdate.IsDisabled
	}

	err := s.sourceRepo.UpdateSource(ctx, g.UID, source)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating source"))
	}

	return source, nil
}

func (s *SourceService) FindSourceByID(ctx context.Context, g *datastore.Group, id string) (*datastore.Source, error) {
	source, err := s.sourceRepo.FindSourceByID(ctx, g.UID, id)

	if err != nil {
		if err == datastore.ErrSourceNotFound {
			return nil, NewServiceError(http.StatusNotFound, err)
		}

		return nil, NewServiceError(http.StatusBadRequest, errors.New("error retrieving source"))
	}

	return source, nil
}

func (s *SourceService) LoadSourcesPaged(ctx context.Context, g *datastore.Group, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	sources, paginationData, err := s.sourceRepo.LoadSourcesPaged(ctx, g.UID, filter, pageable)
	if err != nil {
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching sources"))
	}

	return sources, paginationData, nil
}

func (s *SourceService) DeleteSourceByID(ctx context.Context, g *datastore.Group, id string) error {
	//ToDo: add check here to ensure the source doesn't have any existing subscriptions
	err := s.sourceRepo.DeleteSourceByID(ctx, g.UID, id)

	if err != nil {
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete source"))
	}

	return nil
}
