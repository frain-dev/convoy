package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/retention"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) ListRetentionRuns(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	if !h.A.Licenser.RetentionPolicy() {
		_ = render.Render(w, r, util.NewErrorResponse("retention history is only available with a license key", http.StatusForbidden))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	runs, err := retention.NewRunStore(h.A.DB).List(r.Context(), limit)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retention runs fetched successfully", runs, http.StatusOK))
}

func (h *Handler) GetRetentionRun(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	if !h.A.Licenser.RetentionPolicy() {
		_ = render.Render(w, r, util.NewErrorResponse("retention history is only available with a license key", http.StatusForbidden))
		return
	}

	run, err := retention.NewRunStore(h.A.DB).Get(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		if errors.Is(err, retention.ErrRunNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retention run fetched successfully", run, http.StatusOK))
}
