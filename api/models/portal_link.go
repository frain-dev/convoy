package models

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

type QueryListPortalLink struct {
	// List of endpoint ids
	EndpointIds []string `json:"endpoint_ids"`

	// The owner ID of the endpoint
	OwnerID string `json:"ownerId" example:"01H0JA5MEES38RRK3HTEJC647K"`

	Pageable
}

type QueryListPortalLinkResponse struct {
	datastore.Pageable
	*datastore.FilterBy
}

func (q *QueryListPortalLink) Transform(r *http.Request) *QueryListPortalLinkResponse {
	return &QueryListPortalLinkResponse{
		Pageable: m.GetPageableFromContext(r.Context()),
		FilterBy: &datastore.FilterBy{
			EndpointIDs: getEndpointIDs(r),
			OwnerID:     r.URL.Query().Get("ownerId"),
		},
	}
}
