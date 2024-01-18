package handlers

import (
	"context"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateEndpoint
//
//	@Summary		Create an endpoint
//	@Description	This endpoint creates an endpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpoint	body		models.CreateEndpoint	true	"Endpoint Details"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [post]
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.GetAuthUserFromContext(r.Context())

	var e models.CreateEndpoint

	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = e.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if h.IsReqWithPortalLinkToken(authUser) {
		portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache)
		pLink, err := portalLinkRepo.FindPortalLinkByToken(r.Context(), authUser.Credential.Token)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		e.OwnerID = pLink.OwnerID
	}

	ce := services.CreateEndpointService{
		Cache:          h.A.Cache,
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		ProjectRepo:    postgres.NewProjectRepo(h.A.DB, h.A.Cache),
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache),
		E:              e,
		ProjectID:      project.UID,
	}

	endpoint, err := ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint created successfully", resp, http.StatusCreated))
}

// GetEndpoint
//
//	@Summary		Retrieve endpoint
//	@Description	This endpoint fetches an endpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [get]
func (h *Handler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", resp, http.StatusOK))
}

// GetEndpoints
//
//	@Summary		List all endpoints
//	@Description	This endpoint fetches an endpoints
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			request		query		models.QueryListEndpoint	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=pagedResponse{content=[]models.EndpointResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [get]
func (h *Handler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var q *models.QueryListEndpoint
	data := q.Transform(r)

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		portalLink, err := h.retrievePortalLinkFromToken(r)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		endpointIDs, err := h.getEndpoints(r, portalLink)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		if len(endpointIDs) == 0 {
			_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
				pagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	endpoints, paginationData, err := postgres.NewEndpointRepo(h.A.DB, h.A.Cache).LoadEndpointsPaged(r.Context(), project.UID, data.Filter, data.Pageable)
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to load endpoints")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := models.NewListResponse(endpoints, func(endpoint datastore.Endpoint) models.EndpointResponse {
		return models.EndpointResponse{Endpoint: &endpoint}
	})
	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully",
		pagedResponse{Content: &resp, Pagination: &paginationData}, http.StatusOK))
}

// UpdateEndpoint
//
//	@Summary		Update an endpoint
//	@Description	This endpoint updates an endpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpointID	path		string					true	"Endpoint ID"
//	@Param			endpoint	body		models.UpdateEndpoint	true	"Endpoint Details"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [put]
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var e models.UpdateEndpoint

	err = util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = e.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ce := services.UpdateEndpointService{
		Cache:        h.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		ProjectRepo:  postgres.NewProjectRepo(h.A.DB, h.A.Cache),
		E:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint updated successfully", resp, http.StatusAccepted))
}

// DeleteEndpoint
//
//	@Summary		Delete endpoint
//	@Description	This endpoint deletes an endpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [delete]
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = postgres.NewEndpointRepo(h.A.DB, h.A.Cache).DeleteEndpoint(r.Context(), endpoint, project.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete endpoint")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete endpoint", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

// ExpireSecret
//
//	@Summary		Roll endpoint secret
//	@Description	This endpoint expires and re-generates the endpoint secret.
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			endpointID	path		string				true	"Endpoint ID"
//	@Param			endpoint	body		models.ExpireSecret	true	"Expire Secret Body Parameters"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/expire_secret [put]
func (h *Handler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var e *models.ExpireSecret
	err = util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	xs := services.ExpireSecretService{
		Queuer:       h.A.Queue,
		Cache:        h.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		ProjectRepo:  postgres.NewProjectRepo(h.A.DB, h.A.Cache),
		S:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = xs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		resp, http.StatusOK))
}

// PauseEndpoint
//
//	@Summary		Pause endpoint
//	@Description	This endpoint toggles an endpoint status between the active and paused states
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/pause [put]
func (h *Handler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ps := services.PauseEndpointService{
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}

	endpoint, err := ps.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", resp, http.StatusAccepted))
}

func (h *Handler) retrieveEndpoint(ctx context.Context, endpointID, projectID string) (*datastore.Endpoint, error) {
	endpointRepo := postgres.NewEndpointRepo(h.A.DB, h.A.Cache)
	return endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
}
