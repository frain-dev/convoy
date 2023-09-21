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

	resp := models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", resp, http.StatusOK))
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

	if !portalLink.CanManageEndpoint {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	var e models.CreateEndpoint
	err = util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Automatically add ownerID
	e.OwnerID = portalLink.OwnerID

	ce := services.CreateEndpointService{
		Cache:          a.A.Cache,
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:    postgres.NewProjectRepo(a.A.DB),
		PortalLinkRepo: postgres.NewPortalLinkRepo(a.A.DB),
		E:              e,
		ProjectID:      portalLink.ProjectID,
	}

	endpoint, err := ce.Run(r.Context())
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

	ce := services.UpdateEndpointService{
		Cache:        a.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB),
		E:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = ce.Run(r.Context())
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

	err = postgres.NewEndpointRepo(a.A.DB).DeleteEndpoint(r.Context(), endpoint, project.UID)
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

	xs := services.ExpireSecretService{
		Queuer:       a.A.Queue,
		Cache:        a.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB),
		S:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = xs.Run(r.Context())
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

	ps := services.PauseEndpointService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}
	endpoint, err := ps.Run(r.Context())
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
