package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (dh *DashboardHandler) UpdateOrganisationMembership(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("membership updated successfully", nil, http.StatusCreated))
}
