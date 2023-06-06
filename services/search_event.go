package services

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/log"
)

type SearchEventService struct {
	EventRepo datastore.EventRepository
	Searcher  searcher.Searcher

	Filter *datastore.Filter
}

func (e *SearchEventService) Run(ctx context.Context) ([]datastore.Event, datastore.PaginationData, error) {
	var events []datastore.Event
	ids, paginationData, err := e.Searcher.Search(e.Filter.Project.UID, &datastore.SearchFilter{
		Query: e.Filter.Query,
		FilterBy: datastore.FilterBy{
			EndpointID:   e.Filter.EndpointID,
			SourceID:     e.Filter.SourceID,
			ProjectID:    e.Filter.Project.UID,
			SearchParams: e.Filter.SearchParams,
		},
		Pageable: e.Filter.Pageable,
	})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events from search backend")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: err.Error()}
	}

	if len(ids) == 0 {
		return events, paginationData, nil
	}

	events, err = e.EventRepo.FindEventsByIDs(ctx, e.Filter.Project.UID, ids)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events from event ids")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: err.Error()}
	}

	return events, paginationData, err
}
