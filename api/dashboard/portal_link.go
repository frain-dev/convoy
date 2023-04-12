package dashboard

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

func createPortalLinkService(a *DashboardHandler) *services.PortalLinkService {
	portalRepo := postgres.NewPortalLinkRepo(a.A.DB)
	endpointService := createEndpointService(a)

	return services.NewPortalLinkService(portalRepo, endpointService)
}

func (a *DashboardHandler) CreatePortalLink(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) GetPortalLinkByID(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) UpdatePortalLink(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) RevokePortalLink(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) LoadPortalLinksPaged(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) CreatePortalLinkEndpoint(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) GetPortalLinkEndpoints(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) GetPortalLinkDevices(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpointIDs := m.GetEndpointIDsFromContext(r)

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

func (a *DashboardHandler) GetPortalLinkKeys(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())
	endpointIDs := m.GetEndpointIDsFromContext(r)

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
