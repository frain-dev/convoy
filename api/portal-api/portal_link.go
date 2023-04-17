package portalapi

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createPortalLinkService(a *PortalLinkHandler) *services.PortalLinkService {
	portalRepo := postgres.NewPortalLinkRepo(a.A.DB)
	endpointService := createEndpointService(a)

	return services.NewPortalLinkService(portalRepo, endpointService)
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

	portalLinkService := createPortalLinkService(a)
	endpoints, err := portalLinkService.GetPortalLinkEndpoints(r.Context(), portalLink, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully", endpoints, http.StatusOK))
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
