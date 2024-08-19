package handlers

import (
	"net/http"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", h.A.Licenser.FeatureListJSON(), http.StatusOK))
}
