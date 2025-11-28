package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/migrations"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

// CreatePortalLink
//
//	@Summary		Create a portal link
//	@Description	This endpoint creates a portal link
//	@Tags			Portal Links
//	@Id				CreatePortalLink
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string							true	"Project ID"
//	@Param			portallink	body		models.CreatePortalLinkRequest	true	"Portal Link Details"
//	@Success		201			{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links [post]
func (h *Handler) CreatePortalLink(w http.ResponseWriter, r *http.Request) {
	err := h.RM.VersionRequest(r, "CreatePortalLink")
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Version request failed for CreatePortalLink: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request", http.StatusBadRequest))
		return
	}

	var newPortalLink models.CreatePortalLinkRequest
	if err := util.ReadJSON(r, &newPortalLink); err != nil {
		h.A.Logger.WithError(err).Errorf("Failed to parse portal link creation request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(newPortalLink.AuthType) {
		// makes sure that the auth type is set for backward compatibility
		newPortalLink.SetDefaultAuthType()
	}

	if err := newPortalLink.Validate(); err != nil {
		h.A.Logger.WithError(err).Errorf("Portal link creation validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// Set context if endpoints are provided - business logic will update endpoint owner_ids
	ctx := r.Context()
	if len(newPortalLink.Endpoints) > 0 {
		ctx = migrations.SetUpdateEndpointOwnerID(ctx, true, newPortalLink.Endpoints)
	}

	cp := services.CreatePortalLinkService{
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB),
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB),
		Portal:         &newPortalLink,
		Project:        project,
	}

	portalLink, err := cp.Run(ctx)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// Fetch the portal link to populate calculated fields like EndpointCount
	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
	portalLink, err = portalLinkRepo.FindPortalLinkByID(ctx, project.UID, portalLink.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link created successfully", pl, http.StatusCreated))
}

// GeneratePortalToken
//
//	@Summary		Updates a portal link auth token
//	@Description	This endpoint retrieves a portal link by its id.
//	@Tags			Portal Links
//	@Id				GeneratePortalToken
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			portalLinkID	path		string	true	"portal link id"
//	@Success		200				{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID} [post]
func (h *Handler) GeneratePortalToken(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var pLink *datastore.PortalLink
	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		pLink, err = h.retrievePortalLinkFromToken(r)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
			return
		}
	} else {
		portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
		pLink, err = portalLinkRepo.FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
		if err != nil {
			if err == datastore.ErrPortalLinkNotFound {
				h.A.Logger.WithError(err).Errorf("Portal link not found: %v", err)
				_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
			return
		}
	}

	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
	err = portalLinkRepo.UpdatePortalLink(r.Context(), project.UID, pLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(pLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link fetched successfully", pl, http.StatusOK))
}

// GetPortalLink
//
//	@Summary		Retrieve a portal link
//	@Description	This endpoint retrieves a portal link by its id.
//	@Tags			Portal Links
//	@Id				GetPortalLink
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			portalLinkID	path		string	true	"portal link id"
//	@Success		200				{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID} [get]
func (h *Handler) GetPortalLink(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var pLink *datastore.PortalLink
	pLink, err = h.retrievePortalLinkFromToken(r)
	if err != nil {
		portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
		pLink, err = portalLinkRepo.FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
		if err != nil {
			if err == datastore.ErrPortalLinkNotFound {
				h.A.Logger.WithError(err).Errorf("Portal link not found: %v", err)
				_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
			return
		}
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(pLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link fetched successfully", pl, http.StatusOK))
}

// UpdatePortalLink
//
//	@Summary		Update a portal link
//	@Description	This endpoint updates a portal link
//	@Id				UpdatePortalLink
//	@Tags			Portal Links
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string							true	"Project ID"
//	@Param			portalLinkID	path		string							true	"portal link id"
//	@Param			portallink		body		models.UpdatePortalLinkRequest	true	"Portal Link Details"
//	@Success		202				{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID} [put]
func (h *Handler) UpdatePortalLink(w http.ResponseWriter, r *http.Request) {
	err := h.RM.VersionRequest(r, "UpdatePortalLink")
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Version request failed for UpdatePortalLink: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request", http.StatusBadRequest))
		return
	}

	var updatePortalLink models.UpdatePortalLinkRequest
	err = util.ReadJSON(r, &updatePortalLink)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Failed to parse portal link update request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(updatePortalLink.AuthType) {
		// makes sure that the auth type is set for backward compatibility
		updatePortalLink.SetDefaultAuthType()
	}

	if err := updatePortalLink.Validate(); err != nil {
		h.A.Logger.WithError(err).Errorf("Portal link update validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := postgres.NewPortalLinkRepo(h.A.DB).FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if errors.Is(err, datastore.ErrPortalLinkNotFound) {
			h.A.Logger.WithError(err).Errorf("Portal link not found during update: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	// Set context if endpoints are provided - business logic will update endpoint owner_ids
	ctx := r.Context()
	if len(updatePortalLink.Endpoints) > 0 {
		ctx = migrations.SetUpdateEndpointOwnerID(ctx, true, updatePortalLink.Endpoints)
	}

	upl := services.UpdatePortalLinkService{
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB),
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB),
		Project:        project,
		Update:         &updatePortalLink,
		PortalLink:     portalLink,
	}

	portalLink, err = upl.Run(ctx)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// Fetch the portal link to populate calculated fields like EndpointCount
	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
	portalLink, err = portalLinkRepo.FindPortalLinkByID(ctx, project.UID, portalLink.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link updated successfully", pl, http.StatusAccepted))
}

// RefreshPortalLinkAuthToken
//
//	@Summary		Get a portal link auth token
//	@Description	This endpoint retrieves a portal link auth token
//	@Id				RefreshPortalLinkAuthToken
//	@Tags			Portal Links
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			portalLinkID	path		string	true	"portal link id"
//	@Success		200				{object}	util.ServerResponse{data=string}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID}/refresh_token [get]
func (h *Handler) RefreshPortalLinkAuthToken(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var pLink *datastore.PortalLink

	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
	pLink, err = portalLinkRepo.RefreshPortalLinkAuthToken(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			h.A.Logger.WithError(err).Errorf("Portal link not found during token refresh: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Portal link fetched successfully", pLink.AuthKey, http.StatusOK))
}

// RevokePortalLink
//
//	@Summary		Revoke a portal link
//	@Description	This endpoint revokes a portal link
//	@Id				RevokePortalLink
//	@Tags			Portal Links
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			portalLinkID	path		string	true	"portal link id"
//	@Success		200				{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID}/revoke [put]
func (h *Handler) RevokePortalLink(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
	portalLink, err := portalLinkRepo.FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if errors.Is(err, datastore.ErrPortalLinkNotFound) {
			h.A.Logger.WithError(err).Errorf("Portal link not found during revocation: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	err = portalLinkRepo.RevokePortalLink(r.Context(), project.UID, portalLink.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to revoke portal link")
		_ = render.Render(w, r, util.NewErrorResponse("failed to revoke portal link", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Portal link revoked successfully", nil, http.StatusOK))
}

// LoadPortalLinksPaged
//
//	@Summary		List all portal links
//	@Description	This endpoint fetches multiple portal links
//	@Tags			Portal Links
//	@Id				LoadPortalLinksPaged
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			request		query		models.QueryListEndpoint	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.PortalLinkResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links [get]
func (h *Handler) LoadPortalLinksPaged(w http.ResponseWriter, r *http.Request) {
	pageable := middleware.GetPageableFromContext(r.Context())
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var q *models.QueryListPortalLink
	data := q.Transform(r)

	portalLinks, paginationData, err := postgres.NewPortalLinkRepo(h.A.DB).LoadPortalLinksPaged(r.Context(), project.UID, data.FilterBy, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Println("an error occurred while fetching portal links")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching portal links", http.StatusBadRequest))
		return
	}

	var plResponse []models.PortalLinkResponse
	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	for _, portalLink := range portalLinks {
		pl := portalLinkResponse(&portalLink, baseUrl)
		plResponse = append(plResponse, pl)
	}

	resp := models.NewListResponse(plResponse, func(p models.PortalLinkResponse) models.PortalLinkResponse {
		return p
	})
	_ = render.Render(w, r, util.NewServerResponse("Portal links fetched successfully", models.PagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func portalLinkResponse(pl *datastore.PortalLink, baseUrl string) models.PortalLinkResponse {
	url := fmt.Sprintf("%s/portal?token=%s", baseUrl, pl.Token)
	if len(pl.AuthKey) > 0 {
		url = fmt.Sprintf("%s&auth_token=%s", url, pl.AuthKey)
	}

	return models.PortalLinkResponse{
		UID:               pl.UID,
		ProjectID:         pl.ProjectID,
		Name:              pl.Name,
		URL:               url,
		Token:             pl.Token,
		OwnerID:           pl.OwnerID,
		Endpoints:         pl.Endpoints,
		EndpointCount:     pl.EndpointCount,
		EndpointsMetadata: pl.EndpointsMetadata,
		CanManageEndpoint: pl.CanManageEndpoint,
		CreatedAt:         pl.CreatedAt,
		UpdatedAt:         pl.UpdatedAt,
		AuthType:          pl.AuthType,
		AuthKey:           pl.AuthKey,
	}
}

func (h *Handler) getEndpoints(r *http.Request, pl *datastore.PortalLink) ([]string, error) {
	results := make([]string, 0)
	if !util.IsStringEmpty(pl.OwnerID) {
		endpointRepo := postgres.NewEndpointRepo(h.A.DB)
		endpoints, err := endpointRepo.FindEndpointsByOwnerID(r.Context(), pl.ProjectID, pl.OwnerID)
		if err != nil {
			return nil, err
		}

		for _, endpoint := range endpoints {
			results = append(results, endpoint.UID)
		}

		return results, nil
	}

	if pl.Endpoints == nil {
		return results, nil
	}

	return pl.Endpoints, nil
}
