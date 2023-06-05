package portalapi

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createPortalLinkService(a *PortalLinkHandler) *services.PortalLinkService {
	portalRepo := postgres.NewPortalLinkRepo(a.A.DB)
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)

	return services.NewPortalLinkService(portalRepo, endpointRepo, a.A.Cache, projectRepo)
}

func (a *PortalLinkHandler) GetPortalLinkEndpoints(w http.ResponseWriter, r *http.Request) {
	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpoints, err := postgres.NewEndpointRepo(a.A.DB).FindEndpointsByID(r.Context(), portalLink.Endpoints, project.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching endpoints")
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch portal link endpoints", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully", endpoints, http.StatusOK))
}

func (a *PortalLinkHandler) GetPortalLink(w http.ResponseWriter, r *http.Request) {
	portalLink, err := a.retrievePortalLink(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServerResponse(err.Error(), nil, http.StatusBadRequest))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServerResponse(err.Error(), nil, http.StatusBadRequest))
		return
	}

	pl := portalLinkResponse(portalLink, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Portal Link fetched successfully", pl, http.StatusOK))
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
		CanManageEndpoint: pl.CanManageEndpoint,
		EndpointsMetadata: pl.EndpointsMetadata,
		CreatedAt:         pl.CreatedAt,
		UpdatedAt:         pl.UpdatedAt,
	}
}
