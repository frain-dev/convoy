package portalapi

import (
	"net/http"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (a *PortalLinkHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", project, http.StatusOK))
}
