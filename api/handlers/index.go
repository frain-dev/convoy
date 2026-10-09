package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/indexes"
	"github.com/frain-dev/convoy/internal/pkg/partitions"
	"github.com/frain-dev/convoy/util"
)

// indexReport is what the admin page reads: what is broken now, and what is owed
// a rebuild.
type indexReport struct {
	Invalid []invalidIndex `json:"invalid"`
	Dropped []droppedIndex `json:"dropped"`
}

type invalidIndex struct {
	Table string `json:"table"`
	Name  string `json:"name"`
	Busy  bool   `json:"busy"`
}

type droppedIndex struct {
	Table         string  `json:"table"`
	Name          string  `json:"name"`
	DroppedAt     string  `json:"dropped_at"`
	BlockedAt     *string `json:"blocked_at,omitempty"`
	BlockedReason string  `json:"blocked_reason,omitempty"`

	// Unique is the one cost beyond speed, so the client does not have to parse
	// the definition to know a rebuild is urgent. The definition itself is not
	// sent: it is SQL the server executes, and the client has no use for it.
	Unique bool `json:"unique"`
}

// ListIndexes reports indexes Postgres left invalid and indexes a migration
// dropped without rebuilding.
//
// Instance admin, and deliberately not gated on the retention license the
// partition routes check. An index build interrupted mid-upgrade leaves the same
// invalid index on any instance, licensed or not, and an operator who cannot see
// that has no way to find out short of querying the catalog by hand.
func (h *Handler) ListIndexes(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	invalid, err := indexes.ListInvalid(r.Context(), h.A.DB.GetConn())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	dropped, err := indexes.ListDropped(r.Context(), h.A.DB.GetConn())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	report := indexReport{
		Invalid: make([]invalidIndex, 0, len(invalid)),
		Dropped: make([]droppedIndex, 0, len(dropped)),
	}
	for _, i := range invalid {
		report.Invalid = append(report.Invalid, invalidIndex{Table: i.Table, Name: i.Name, Busy: i.Busy})
	}
	for _, d := range dropped {
		var blockedAt *string
		if d.BlockedAt != nil {
			formatted := d.BlockedAt.Format(indexTimeFormat)
			blockedAt = &formatted
		}
		report.Dropped = append(report.Dropped, droppedIndex{
			Table:         d.Table,
			Name:          d.Name,
			DroppedAt:     d.DroppedAt.Format(indexTimeFormat),
			BlockedAt:     blockedAt,
			BlockedReason: d.BlockedReason,
			Unique:        d.Unique(),
		})
	}

	_ = render.Render(w, r, util.NewServerResponse("Indexes fetched successfully", report, http.StatusOK))
}

// indexTimeFormat matches what the dashboard's date pipe parses.
const indexTimeFormat = "2006-01-02T15:04:05Z07:00"

// StartIndexRebuild begins a rebuild and returns the run to poll.
//
// A rebuild is hours of work on a large table, so it shares the partition
// runner's row and its instance-wide single-active slot rather than running
// unbounded alongside a conversion of the same table.
func (h *Handler) StartIndexRebuild(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	var request struct {
		Index string `json:"index"`
	}
	if err := util.ReadJSON(r, &request); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	// Trimmed before the lookup so a name that only differs by whitespace does
	// not read as unknown. Nothing else is normalised: the name is matched
	// against the recorded row, never interpolated into SQL.
	index := strings.TrimSpace(request.Index)
	if index == "" {
		_ = render.Render(w, r, util.NewErrorResponse("an index name is required", http.StatusBadRequest))
		return
	}

	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusUnauthorized))
		return
	}

	run, err := partitions.New(h.A.DB, h.A.Logger).StartIndexRebuild(r.Context(), index, user.UID)
	if err != nil {
		// A run already in flight is the caller's cue to watch it rather than an
		// error in what they asked for.
		if errors.Is(err, partitions.ErrRunInProgress) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
			return
		}
		// The name identifies nothing to rebuild, which the caller can act on.
		if errors.Is(err, indexes.ErrNotDropped) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}
		// Nothing would stop a second run, and the message names the index to
		// rebuild first.
		if errors.Is(err, partitions.ErrGuardMissing) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Index rebuild started", run, http.StatusAccepted))
}
