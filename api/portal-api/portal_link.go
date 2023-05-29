package portalapi

import (
	"net/http"

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
