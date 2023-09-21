package models

import (
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"net/http"
)

type QueryListMetaEvent struct {
	SearchParams
	Pageable
}

type QueryListMetaEventResponse struct {
	*datastore.Filter
}

func (ql *QueryListMetaEvent) Transform(r *http.Request) (*QueryListMetaEventResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryListMetaEventResponse{
		Filter: &datastore.Filter{
			SearchParams: searchParams,
			Pageable:     m.GetPageableFromContext(r.Context()),
		},
	}, nil
}

type MetaEventResponse struct {
	*datastore.MetaEvent
}
