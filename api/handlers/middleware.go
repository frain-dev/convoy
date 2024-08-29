package handlers

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

var ErrProjectDisabled = errors.New("this project has been disabled for write operations until you re-upgrade your convoy instance")

func (h *Handler) RequireEnabledProject() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := h.retrieveProject(r)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("failed to retrieve project", http.StatusBadRequest))
				return
			}

			if p.DisabledByLicense {
				_ = render.Render(w, r, util.NewErrorResponse(ErrProjectDisabled.Error(), http.StatusBadRequest))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
