package handlers

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/middleware"
)

// CreatePortalLink
//
//	@Summary		Create a portal link
//	@Description	This endpoint creates a portal link
//	@Tags			Portal Links
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			portallink	body		models.PortalLink	true	"Portal Link Details"
//	@Success		200			{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links [post]
func (h *Handler) CreatePortalLink(w http.ResponseWriter, r *http.Request) {
	var newPortalLink models.PortalLink
	if err := util.ReadJSON(r, &newPortalLink); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	cp := services.CreatePortalLinkService{
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache),
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		Portal:         &newPortalLink,
		Project:        project,
	}

	portalLink, err := cp.Run(r.Context())
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

// GetPortalLink
//
//	@Summary		Retrieve a portal link
//	@Description	This endpoint retrieves a portal link by its id.
//	@Tags			Portal Links
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
	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		pLink, err = h.retrievePortalLinkFromToken(r)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
			return
		}
	} else {
		portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache)
		pLink, err = portalLinkRepo.FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
		if err != nil {
			if err == datastore.ErrPortalLinkNotFound {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
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
//	@Tags			Portal Links
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string				true	"Project ID"
//	@Param			portalLinkID	path		string				true	"portal link id"
//	@Param			portallink		body		models.PortalLink	true	"Portal Link Details"
//	@Success		200				{object}	util.ServerResponse{data=models.PortalLinkResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/portal-links/{portalLinkID} [put]
func (h *Handler) UpdatePortalLink(w http.ResponseWriter, r *http.Request) {
	var updatePortalLink models.PortalLink
	err := util.ReadJSON(r, &updatePortalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache).FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	upl := services.UpdatePortalLinkService{
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache),
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		Project:        project,
		Update:         &updatePortalLink,
		PortalLink:     portalLink,
	}

	portalLink, err = upl.Run(r.Context())
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

// RevokePortalLink
//
//	@Summary		Revoke a portal link
//	@Description	This endpoint revokes a portal link
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

	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache)
	portalLink, err := portalLinkRepo.FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
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
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			perPage		query		string	false	"results per page"
//	@Param			page		query		string	false	"page number"
//	@Param			sort		query		string	false	"sort order"
//	@Success		200			{object}	util.ServerResponse{data=pagedResponse{content=[]models.PortalLinkResponse}}
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

	endpointIDs := getEndpointIDs(r)
	filter := &datastore.FilterBy{EndpointIDs: endpointIDs}

	portalLinks, paginationData, err := postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache).LoadPortalLinksPaged(r.Context(), project.UID, filter, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Println("an error occurred while fetching portal links")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching portal links", http.StatusBadRequest))
		return
	}

	plResponse := []*models.PortalLinkResponse{}
	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	for _, portalLink := range portalLinks {
		pl := portalLinkResponse(&portalLink, baseUrl)
		plResponse = append(plResponse, pl)
	}

	_ = render.Render(w, r, util.NewServerResponse("Portal links fetched successfully", pagedResponse{Content: plResponse, Pagination: &paginationData}, http.StatusOK))
}

func portalLinkResponse(pl *datastore.PortalLink, baseUrl string) *models.PortalLinkResponse {
	return &models.PortalLinkResponse{
		UID:               pl.UID,
		ProjectID:         pl.ProjectID,
		Name:              pl.Name,
		URL:               fmt.Sprintf("%s/portal?token=%s", baseUrl, pl.Token),
		Token:             pl.Token,
		OwnerID:           pl.OwnerID,
		Endpoints:         pl.Endpoints,
		EndpointCount:     len(pl.EndpointsMetadata),
		EndpointsMetadata: pl.EndpointsMetadata,
		CanManageEndpoint: pl.CanManageEndpoint,
		CreatedAt:         pl.CreatedAt,
		UpdatedAt:         pl.UpdatedAt,
	}
}

func (h *Handler) getEndpoints(r *http.Request, pl *datastore.PortalLink) ([]string, error) {
	results := make([]string, 0)
	if !util.IsStringEmpty(pl.OwnerID) {
		endpointRepo := postgres.NewEndpointRepo(h.A.DB, h.A.Cache)
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
