package api

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createPortalLinkService(a *ApplicationHandler) *services.PortalLinkService {
	portalRepo := postgres.NewPortalLinkRepo(a.A.DB)
	endpointService := createEndpointService(a)

	return services.NewPortalLinkService(portalRepo, endpointService)
}

// CreatePortalLink
// @Summary Create a portal link
// @Description This endpoint creates a portal link
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param portallink body models.PortalLink true "Portal Link Details"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/portal-links [post]
func (a *ApplicationHandler) CreatePortalLink(w http.ResponseWriter, r *http.Request) {
	var newPortalLink models.PortalLink
	if err := util.ReadJSON(r, &newPortalLink); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())

	portalLinkService := createPortalLinkService(a)
	portalLink, err := portalLinkService.CreatePortalLink(r.Context(), &newPortalLink, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal link created successfully", pl, http.StatusCreated))
}

// GetPortalLinkByID
// @Summary Get a portal link
// @Description This endpoint fetches a portal link by its id
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param portalLinkID path string true "portal link id"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/portal-links/{portalLinkID} [get]
func (a *ApplicationHandler) GetPortalLinkByID(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())

	portalLinkService := createPortalLinkService(a)
	portalLink, err := portalLinkService.FindPortalLinkByID(r.Context(), project, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	pl := portalLinkResponse(portalLink, baseUrl)

	_ = render.Render(w, r, util.NewServerResponse("Portal link fetched successfully", pl, http.StatusOK))
}

// UpdatePortalLink
// @Summary Update a portal link
// @Description This endpoint updates a portal link
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param portalLinkID path string true "portal link id"
// @Param portallink body models.PortalLink true "Portal Link Details"
// @Success 200 {object} util.ServerResponse{data=models.PortalLinkResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/portal-links/{portalLinkID} [put]
func (a *ApplicationHandler) UpdatePortalLink(w http.ResponseWriter, r *http.Request) {
	var updatePortalLink models.PortalLink
	err := util.ReadJSON(r, &updatePortalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())
	portalLinkService := createPortalLinkService(a)

	portalLink, err := portalLinkService.FindPortalLinkByID(r.Context(), project, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	portalLink, err = portalLinkService.UpdatePortalLink(r.Context(), project, &updatePortalLink, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	pl := portalLinkResponse(portalLink, baseUrl)

	_ = render.Render(w, r, util.NewServerResponse("Portal link updated successfully", pl, http.StatusAccepted))
}

// RevokePortalLink
// @Summary Revoke Portal Link
// @Description This endpoint revokes a portal link
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param portalLinkID path string true "portal link id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/portal-links/{portalLinkID}/revoke [put]
func (a *ApplicationHandler) RevokePortalLink(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	portalLinkService := createPortalLinkService(a)

	portalLink, err := portalLinkService.FindPortalLinkByID(r.Context(), project, chi.URLParam(r, "portalLinkID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = portalLinkService.RevokePortalLink(r.Context(), project, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Portal link revoked successfully", nil, http.StatusOK))
}

// LoadPortalLinksPaged
// @Summary Fetch multiple portal links
// @Description This endpoint fetches multiple portal links
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.PortalLinkResponse}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/portal-links [get]
func (a *ApplicationHandler) LoadPortalLinksPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpointID := m.GetEndpointIDFromContext(r)

	filter := &datastore.FilterBy{EndpointID: endpointID}

	portalLinkService := createPortalLinkService(a)
	portalLinks, paginationData, err := portalLinkService.LoadPortalLinksPaged(r.Context(), project, filter, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	plResponse := []*models.PortalLinkResponse{}
	baseUrl := m.GetHostFromContext(r.Context())

	for _, portalLink := range portalLinks {
		pl := portalLinkResponse(&portalLink, baseUrl)
		plResponse = append(plResponse, pl)
	}

	_ = render.Render(w, r, util.NewServerResponse("Portal links fetched successfully", pagedResponse{Content: plResponse, Pagination: &paginationData}, http.StatusOK))
}

// CreatePortalLinkEndpoint
// @Summary Create an endpoint
// @Description This endpoint creates an endpoint
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /portal/endpoints [post]
func (a *ApplicationHandler) CreatePortalLinkEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())
	portalLink := m.GetPortalLinkFromContext(r.Context())
	portalLinkService := createPortalLinkService(a)

	endpoint, err := portalLinkService.CreateEndpoint(r.Context(), project, e, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint created successfully", endpoint, http.StatusCreated))
}

// GetPortalLinkEndpoints
// @Summary Get endpoints
// @Description This endpoint fetches all portal link endpoints
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Success 200 {object} util.ServerResponse{data=[]datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /portal/endpoints [get]
func (a *ApplicationHandler) GetPortalLinkEndpoints(w http.ResponseWriter, r *http.Request) {
	portalLink := m.GetPortalLinkFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())

	portalLinkService := createPortalLinkService(a)
	endpoints, err := portalLinkService.GetPortalLinkEndpoints(r.Context(), portalLink, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully", endpoints, http.StatusOK))
}

// GetPortalLinkDevices
// @Summary Get portal link devices
// @Description This endpoint fetches all portal link devices
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Success 200 {object} util.ServerResponse{data=[]datastore.Device}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /portal/devices [get]
func (a *ApplicationHandler) GetPortalLinkDevices(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpointIDs := m.GetEndpointIDsFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		EndpointIDs: endpointIDs,
	}

	deviceRepo := postgres.NewDeviceRepo(a.A.DB)
	devices, paginationData, err := deviceRepo.LoadDevicesPaged(r.Context(), project.UID, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching devices", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Devices fetched successfully", pagedResponse{Content: &devices, Pagination: &paginationData}, http.StatusOK))
}

// GetPortalLinkKeys
// @Summary Get portal link keys
// @Description This endpoint fetches all portal link endpoints keys
// @Tags PortalLinks
// @Accept  json
// @Produce  json
// @Success 200 {object} util.ServerResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /portal/keys [get]
func (a *ApplicationHandler) GetPortalLinkKeys(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())
	endpointIDs := m.GetEndpointIDsFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		ProjectID:   project.UID,
		EndpointIDs: endpointIDs,
		KeyType:     datastore.CLIKey,
	}

	apiKeyRepo := postgres.NewAPIKeyRepo(a.A.DB)
	apiKeys, paginationData, err := apiKeyRepo.LoadAPIKeysPaged(r.Context(), f, &pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching api keys", http.StatusInternalServerError))
		return
	}

	apiKeyByIDResponse := apiKeyByIDResponse(apiKeys)
	_ = render.Render(w, r, util.NewServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeyByIDResponse, Pagination: &paginationData}, http.StatusOK))
}

func portalLinkResponse(pl *datastore.PortalLink, baseUrl string) *models.PortalLinkResponse {
	return &models.PortalLinkResponse{
		UID:               pl.UID,
		ProjectID:         pl.ProjectID,
		Name:              pl.Name,
		URL:               fmt.Sprintf("%s/portal?token=%s", baseUrl, pl.Token),
		Token:             pl.Token,
		Endpoints:         pl.Endpoints,
		EndpointCount:     len(pl.Endpoints),
		EndpointsMetadata: pl.EndpointsMetadata,
		CreatedAt:         pl.CreatedAt,
		UpdatedAt:         pl.UpdatedAt,
	}
}
