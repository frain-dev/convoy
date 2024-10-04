package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"net/http"
	"time"

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
//	@Id				CreateEndpoint
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpoint	body		models.CreateEndpoint	true	"Endpoint Details"
//	@Success		201			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [post]
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.GetAuthUserFromContext(r.Context())

	err := h.RM.VersionRequest(r, "CreateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var e models.CreateEndpoint

	err = util.ReadJSON(r, &e)
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
		Licenser:       h.A.Licenser,
		E:              e,
		ProjectID:      project.UID,
	}

	endpoint, err := ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	serverResponse := util.NewServerResponse(
		"Endpoint created successfully",
		resp, http.StatusCreated)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "CreateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusCreated)
}

// GetEndpoint
//
//	@Summary		Retrieve endpoint
//	@Description	This endpoint fetches an endpoint
//	@Id				GetEndpoint
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
	serverResponse := util.NewServerResponse(
		"Endpoint fetched successfully", resp, http.StatusOK)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "GetEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusOK)
}

// GetEndpoints
//
//	@Summary		List all endpoints
//	@Description	This endpoint fetches an endpoints
//	@Tags			Endpoints
//	@Id				GetEndpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			request		query		models.QueryListEndpoint	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EndpointResponse}}
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
				models.PagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
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

	// fetch keys from redis and mutate endpoints slice
	keys := make([]string, len(endpoints))
	for i := 0; i < len(endpoints); i++ {
		keys[i] = fmt.Sprintf("breaker:%s", endpoints[i].UID)
	}

	cbs, err := h.A.Redis.MGet(r.Context(), keys...).Result()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	for i := 0; i < len(cbs); i++ {
		if cbs[i] != nil {
			str, ok := cbs[i].(string)
			if ok {
				var c circuit_breaker.CircuitBreaker
				asBytes := []byte(str)
				innerErr := msgpack.DecodeMsgPack(asBytes, &c)
				if innerErr != nil {
					continue
				}
				endpoints[i].FailureRate = c.FailureRate
			}
		}
	}

	resp := models.NewListResponse(endpoints, func(endpoint datastore.Endpoint) models.EndpointResponse {
		return models.EndpointResponse{Endpoint: &endpoint}
	})

	serverResponse := util.NewServerResponse(
		"Endpoints fetched successfully",
		models.PagedResponse{Content: &resp, Pagination: &paginationData}, http.StatusOK)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "GetEndpoints")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusOK)
}

// UpdateEndpoint
//
//	@Summary		Update an endpoint
//	@Description	This endpoint updates an endpoint
//	@Id				UpdateEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpointID	path		string					true	"Endpoint ID"
//	@Param			endpoint	body		models.UpdateEndpoint	true	"Endpoint Details"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [put]
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	err := h.RM.VersionRequest(r, "UpdateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

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
		Licenser:     h.A.Licenser,
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
	serverResponse := util.NewServerResponse("Endpoint updated successfully", resp, http.StatusAccepted)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

// DeleteEndpoint
//
//	@Summary		Delete endpoint
//	@Description	This endpoint deletes an endpoint
//	@Tags			Endpoints
//	@Id				DeleteEndpoint
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
//	@Id				ExpireSecret
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
//	@Description	Toggles an endpoint's status between active and paused states
//	@Id				PauseEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
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
	serverResponse := util.NewServerResponse("endpoint status updated successfully", resp, http.StatusAccepted)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

// ActivateEndpoint
//
//	@Summary		Activate endpoint
//	@Description	Activated an inactive endpoint
//	@Id				PauseEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/activate [post]
func (h *Handler) ActivateEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	aes := services.ActivateEndpointService{
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}

	endpoint, err := aes.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	cbs, err := h.A.Redis.Get(r.Context(), fmt.Sprintf("breaker:%s", endpoint.UID)).Result()
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to find circuit breaker")
	}

	if len(cbs) > 0 {
		var c *circuit_breaker.CircuitBreaker
		asBytes := []byte(cbs)
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			h.A.Logger.WithError(innerErr).Error("failed to decode circuit breaker")
		} else {
			c.ResetCircuitBreaker(time.Now())
			b, msgPackErr := msgpack.EncodeMsgPack(c)
			if msgPackErr != nil {
				h.A.Logger.WithError(msgPackErr).Error("failed to encode circuit breaker")
			}
			h.A.Redis.Set(r.Context(), fmt.Sprintf("breaker:%s", endpoint.UID), b, time.Minute*5)
		}
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	serverResponse := util.NewServerResponse("endpoint status successfully activated", resp, http.StatusAccepted)

	rb, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

func (h *Handler) retrieveEndpoint(ctx context.Context, endpointID, projectID string) (*datastore.Endpoint, error) {
	endpointRepo := postgres.NewEndpointRepo(h.A.DB, h.A.Cache)
	return endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
}
