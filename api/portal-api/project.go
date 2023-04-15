package portalapi

import (
	"net/http"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (a *PortalLinkHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", project, http.StatusOK))
}
