package portalapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/render"
)

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

func (a *PortalLinkHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", endpoint, http.StatusOK))
}

func (a *PortalLinkHandler) retrieveEndpoint(r *http.Request) (*datastore.Endpoint, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	return endpointRepo.FindEndpointByID(r.Context(), endpointID, project.UID)
}
