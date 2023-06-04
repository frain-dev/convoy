package public

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

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createPortalLinkService(a *PublicHandler) *services.PortalLinkService {
	portalRepo := postgres.NewPortalLinkRepo(a.A.DB)
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)

	return services.NewPortalLinkService(portalRepo, endpointRepo, a.A.Cache, projectRepo)
}

// CreatePortalLink
// @Summary Create a portal link
// @Description This endpoint creates a portal link
// @Tags Portal Links
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param portallink body models.PortalLink true "Portal Link Details"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/portal-links [post]
func (a *PublicHandler) CreatePortalLink(w http.ResponseWriter, r *http.Request) {
	var newPortalLink models.PortalLink
	if err := util.ReadJSON(r, &newPortalLink); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLinkService := createPortalLinkService(a)
	portalLink, err := portalLinkService.CreatePortalLink(r.Context(), &newPortalLink, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link created successfully", pl, http.StatusCreated))
}

// GetPortalLinkByID
// @Summary Retrieve a portal link
// @Description This endpoint retrieves a portal link by its id.
// @Tags Portal Links
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param portalLinkID path string true "portal link id"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/portal-links/{portalLinkID} [get]
func (a *PublicHandler) GetPortalLinkByID(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := postgres.NewPortalLinkRepo(a.A.DB).FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link fetched successfully", pl, http.StatusOK))
}

// UpdatePortalLink
// @Summary Update a portal link
// @Description This endpoint updates a portal link
// @Tags Portal Links
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param portalLinkID path string true "portal link id"
// @Param portallink body models.PortalLink true "Portal Link Details"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/portal-links/{portalLinkID} [put]
func (a *PublicHandler) UpdatePortalLink(w http.ResponseWriter, r *http.Request) {
	var updatePortalLink models.PortalLink
	err := util.ReadJSON(r, &updatePortalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err := postgres.NewPortalLinkRepo(a.A.DB).FindPortalLinkByID(r.Context(), project.UID, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving portal link", http.StatusBadRequest))
		return
	}

	portalLinkService := createPortalLinkService(a)
	portalLink, err = portalLinkService.UpdatePortalLink(r.Context(), project, &updatePortalLink, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link updated successfully", pl, http.StatusAccepted))
}

// RevokePortalLink
// @Summary Revoke a portal link
// @Description This endpoint revokes a portal link
// @Tags Portal Links
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param portalLinkID path string true "portal link id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/portal-links/{portalLinkID}/revoke [put]
func (a *PublicHandler) RevokePortalLink(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLinkRepo := postgres.NewPortalLinkRepo(a.A.DB)
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
// @Summary List all portal links
// @Description This endpoint fetches multiple portal links
// @Tags Portal Links
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.PortalLinkResponse}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/portal-links [get]
func (a *PublicHandler) LoadPortalLinksPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointIDs := getEndpointIDs(r)
	filter := &datastore.FilterBy{EndpointIDs: endpointIDs}

	portalLinks, paginationData, err := postgres.NewPortalLinkRepo(a.A.DB).LoadPortalLinksPaged(r.Context(), project.UID, filter, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Println("an error occurred while fetching portal links")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching portal links", http.StatusBadRequest))
		return
	}

	plResponse := []*models.PortalLinkResponse{}
	baseUrl, err := a.retrieveHost()
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
		UID:                pl.UID,
		ProjectID:          pl.ProjectID,
		Name:               pl.Name,
		URL:                fmt.Sprintf("%s/portal?token=%s", baseUrl, pl.Token),
		Token:              pl.Token,
		OwnerID:            pl.OwnerID,
		Endpoints:          pl.Endpoints,
		EndpointCount:      len(pl.EndpointsMetadata),
		EndpointsMetadata:  pl.EndpointsMetadata,
		EndpointManagement: pl.EndpointManagement,
		CreatedAt:          pl.CreatedAt,
		UpdatedAt:          pl.UpdatedAt,
	}
}
