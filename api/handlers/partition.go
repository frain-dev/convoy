package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/partitions"
	"github.com/frain-dev/convoy/util"
)

// StartPartitionRun begins a partition conversion and returns the run to poll.
// Conversion rewrites the table, so this is an instance admin operation gated on
// the same license check the CLI uses.
func (h *Handler) StartPartitionRun(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	if !h.A.Licenser.RetentionPolicy() {
		_ = render.Render(w, r, util.NewErrorResponse("partitioning is only available with a license key", http.StatusForbidden))
		return
	}

	var request struct {
		Table     string `json:"table"`
		Operation string `json:"operation"`
	}
	if err := util.ReadJSON(r, &request); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	table, err := partitions.ParseTable(request.Table)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	operation := partitions.Operation(request.Operation)
	if operation != partitions.OperationPartition && operation != partitions.OperationUnpartition {
		_ = render.Render(w, r, util.NewErrorResponse("operation must be partition or unpartition", http.StatusBadRequest))
		return
	}

	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusUnauthorized))
		return
	}

	run, err := partitions.New(h.A.DB, h.A.Logger).Start(r.Context(), table, operation, user.UID)
	if err != nil {
		// Conflict rather than a generic error: the caller can act on this by
		// waiting for or looking at the run already in flight.
		if errors.Is(err, partitions.ErrRunInProgress) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
			return
		}
		// The table is already in the shape the operation would produce, so the
		// request is the caller's mistake and naming it is more use than a 500.
		if errors.Is(err, partitions.ErrAlreadyPartitioned) || errors.Is(err, partitions.ErrNotPartitioned) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
		// Nothing would stop a second conversion of this table, so the start is
		// refused until the guard is back, and the message says how.
		if errors.Is(err, partitions.ErrGuardMissing) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Partition run started", run, http.StatusAccepted))
}

func (h *Handler) GetPartitionRun(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	run, err := partitions.New(h.A.DB, h.A.Logger).Get(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		if errors.Is(err, partitions.ErrRunNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Partition run fetched successfully", run, http.StatusOK))
}

// ListPartitionTables returns each convertible table's current shape, so a
// caller only offers the operation that would change it.
func (h *Handler) ListPartitionTables(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	states, err := partitions.New(h.A.DB, h.A.Logger).TableStates(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Partition tables fetched successfully", states, http.StatusOK))
}

func (h *Handler) ListPartitionRuns(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	runs, err := partitions.New(h.A.DB, h.A.Logger).List(r.Context(), limit)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Partition runs fetched successfully", runs, http.StatusOK))
}
