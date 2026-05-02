package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type triggerBackupRequest struct {
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

type triggerBackupPayload struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TriggerBackup enqueues an asynchronous manual backup job.
// POST /ui/backups/trigger
func (h *Handler) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	if !h.isInstanceAdmin(r) {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return
	}

	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to load config", http.StatusInternalServerError))
		return
	}

	if !cfg.RetentionPolicy.IsRetentionPolicyEnabled {
		_ = render.Render(w, r, util.NewErrorResponse("backup is not enabled in configuration", http.StatusUnprocessableEntity))
		return
	}

	// Default time window: last backup interval to now
	end := time.Now()
	start := end.Add(-exporter.ParseBackupInterval(cfg.RetentionPolicy.BackupInterval))

	// Parse optional overrides from request body (empty body is fine)
	var req triggerBackupRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("invalid request body", http.StatusBadRequest))
			return
		}
		if req.Start != nil {
			start = *req.Start
		}
		if req.End != nil {
			end = *req.End
		}
	}

	if !start.Before(end) {
		_ = render.Render(w, r, util.NewErrorResponse("start must be before end", http.StatusBadRequest))
		return
	}

	payload, err := json.Marshal(triggerBackupPayload{Start: start, End: end})
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to marshal payload", http.StatusInternalServerError))
		return
	}

	job := &queue.Job{
		ID:      ulid.Make().String(),
		Payload: payload,
	}

	if err := h.A.Queue.Write(r.Context(), convoy.ManualBackupJob, convoy.DefaultQueue, job); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to enqueue backup job", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("backup job enqueued", map[string]interface{}{
		"job_id": job.ID,
		"start":  start.Format(time.RFC3339),
		"end":    end.Format(time.RFC3339),
	}, http.StatusAccepted))
}
