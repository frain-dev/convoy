package portalapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/render"
)

func createEndpointService(a *PortalLinkHandler) *services.EndpointService {
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	eventRepo := postgres.NewEventRepo(a.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)

	return services.NewEndpointService(
		projectRepo, endpointRepo, eventRepo, eventDeliveryRepo, a.A.Cache, a.A.Queue,
	)
}

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

func (a *PortalLinkHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(portalLink.OwnerID) {
		err := errors.New("portal link needs to be scoped to an owner_id")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var e models.Endpoint
	err = util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err := endpointService.CreateEndpoint(r.Context(), e, portalLink.ProjectID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r,
		util.NewServerResponse("Endpoint created successfully", endpoint,
			http.StatusCreated))
}

func (a *PortalLinkHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.UpdateEndpoint

	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err = endpointService.UpdateEndpoint(r.Context(), e, endpoint, project)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r,
		util.NewServerResponse("Endpoint updated successfully", endpoint,
			http.StatusAccepted))
}

func (a *PortalLinkHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(portalLink.OwnerID) {
		err := errors.New("portal link needs to be scoped to an owner_id")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointService := createEndpointService(a)
	err = endpointService.DeleteEndpoint(r.Context(), endpoint, project)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r,
		util.NewServerResponse("Endpoint created successfully", endpoint,
			http.StatusCreated))
}

func (a *PortalLinkHandler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
	var e *models.ExpireSecret
	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err = endpointService.ExpireSecret(r.Context(), e, endpoint, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		endpoint, http.StatusOK))
}

func (a *PortalLinkHandler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointService := createEndpointService(a)
	endpoint, err := endpointService.PauseEndpoint(r.Context(), project.UID, endpointID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", endpoint, http.StatusAccepted))
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
