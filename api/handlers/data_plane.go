package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/dataplanestats"
	"github.com/frain-dev/convoy/util"
)

// GetDataPlaneStatus returns every replica's last published snapshot. The
// dashboard reads this; the numbers are computed by whatever data plane each
// replica runs, and nothing here knows what its stages are called.
func (h *Handler) GetDataPlaneStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrictInstanceAdmin(w, r) {
		return
	}

	// Same nil convention as the queue inspector: no store wired is 501, not an
	// empty list, because an empty list reads as a plane with nothing in it.
	if h.A.DataPlaneMonitor == nil {
		_ = render.Render(w, r, util.NewErrorResponse("data plane monitoring is unavailable on this deployment", http.StatusNotImplemented))
		return
	}

	status, err := h.A.DataPlaneMonitor.DataPlaneStatus(r.Context(), h.dataPlaneStaleAfter())
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to load data plane status", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to load data plane status", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("data plane status fetched successfully", status, http.StatusOK))
}

// GetDataPlaneSnapshot returns this process's own live snapshot. It is served by
// the replica running the plane, so a harness or an operator can read depth
// without waiting for a publish interval to elapse.
func (h *Handler) GetDataPlaneSnapshot(w http.ResponseWriter, r *http.Request) {
	if !h.requireStrictInstanceAdmin(w, r) {
		return
	}

	if h.A.DataPlaneReporter == nil {
		_ = render.Render(w, r, util.NewErrorResponse("this process runs no reporting data plane", http.StatusNotImplemented))
		return
	}

	snapshot, err := h.A.DataPlaneReporter.Snapshot(r.Context())
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to read data plane snapshot", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to read data plane snapshot", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("data plane snapshot fetched successfully", snapshot, http.StatusOK))
}

// dataPlaneStaleAfter derives the staleness threshold from the one interval that
// already owns sampling, so enabling this needs no second knob.
func (h *Handler) dataPlaneStaleAfter() time.Duration {
	return dataplanestats.StaleAfter(dataplanestats.PublishInterval(h.A.Cfg.Metrics.Prometheus.SampleTime))
}
