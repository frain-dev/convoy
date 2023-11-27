package public

import (
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

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

// CreateEndpoint
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
func (a *PublicHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
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

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	ce := services.CreateEndpointService{
		Cache:          a.A.Cache,
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB, a.A.Cache),
		ProjectRepo:    postgres.NewProjectRepo(a.A.DB, a.A.Cache),
		PortalLinkRepo: postgres.NewPortalLinkRepo(a.A.DB, a.A.Cache),
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
func (a *PublicHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", resp, http.StatusOK))
}

// GetEndpoints
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
func (a *PublicHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEndpoint
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data := q.Transform(r)
	endpoints, paginationData, err := postgres.NewEndpointRepo(a.A.DB, a.A.Cache).LoadEndpointsPaged(r.Context(), project.UID, data.Filter, data.Pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load endpoints")
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
func (a *PublicHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.UpdateEndpoint

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

	err = e.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ce := services.UpdateEndpointService{
		Cache:        a.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB, a.A.Cache),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB, a.A.Cache),
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
func (a *PublicHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
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

	err = postgres.NewEndpointRepo(a.A.DB, a.A.Cache).DeleteEndpoint(r.Context(), endpoint, project.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete endpoint")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete endpoint", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

// ExpireSecret
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
func (a *PublicHandler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
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
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB, a.A.Cache),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB, a.A.Cache),
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

// ToggleEndpointStatus
//	@Summary		Toggle endpoint status
//	@Description	This endpoint toggles an endpoint status between the active and inactive statetes
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/toggle_status [put]
func (a *PublicHandler) ToggleEndpointStatus(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	te := services.ToggleEndpointStatusService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB, a.A.Cache),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}

	endpoint, err := te.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", resp, http.StatusAccepted))
}

// PauseEndpoint
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
func (a *PublicHandler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	ps := services.PauseEndpointService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB, a.A.Cache),
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

func (a *PublicHandler) retrieveEndpoint(r *http.Request) (*datastore.Endpoint, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointRepo := postgres.NewEndpointRepo(a.A.DB, a.A.Cache)
	return endpointRepo.FindEndpointByID(r.Context(), endpointID, project.UID)
}
