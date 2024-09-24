package handlers

import (
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	v, err := h.A.Licenser.FeatureListJSON(r.Context())
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to get license features")
		_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
}
