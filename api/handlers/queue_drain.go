package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/queue/drain"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) queueController(w http.ResponseWriter, r *http.Request) (*drain.Controller, bool) {
	if !h.requireStrictInstanceAdmin(w, r) {
		return nil, false
	}
	c := h.A.QueueDrain
	if h.A.PreviousQueueDrain != nil && chi.URLParam(r, "storeID") == h.A.PreviousQueueDrain.Target.StoreID {
		c = h.A.PreviousQueueDrain
	}
	if c == nil {
		_ = render.Render(w, r, util.NewErrorResponse("Queue drain is not configured for this deployment.", http.StatusNotImplemented))
		return nil, false
	}
	if id := chi.URLParam(r, "storeID"); id != "" && id != c.Target.StoreID {
		_ = render.Render(w, r, util.NewErrorResponse("Queue store is not configured for execution.", http.StatusNotFound))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return c, true
}

func (h *Handler) GetQueueDrain(w http.ResponseWriter, r *http.Request) {
	c, ok := h.queueController(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	status, err := c.Status(ctx)
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Queue operation status", status, http.StatusOK))
}

func (h *Handler) BeginQueueDrain(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	c, ok := h.queueController(w, r)
	if !ok {
		return
	}
	var request struct {
		ResumePaused          bool          `json:"resume_paused"`
		Purpose               drain.Purpose `json:"purpose"`
		ConfigurationRevision string        `json:"configuration_revision"`
		IdempotencyKey        string        `json:"idempotency_key"`
	}
	if !decodeQueueCommand(w, r, &request) {
		return
	}
	user, err := h.retrieveUser(r)
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	op, err := c.BeginWithOptions(drain.WithActor(r.Context(), "user:"+user.UID), request.Purpose, request.IdempotencyKey, request.ConfigurationRevision, request.ResumePaused)
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Queue operation accepted", op, http.StatusAccepted))
}

func (h *Handler) CommandQueueDrain(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	c, ok := h.queueController(w, r)
	if !ok {
		return
	}
	var request struct {
		Revision int64  `json:"revision"`
		Command  string `json:"command"`
	}
	if !decodeQueueCommand(w, r, &request) {
		return
	}
	user, err := h.retrieveUser(r)
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	op, err := c.Command(drain.WithActor(r.Context(), "user:"+user.UID), chi.URLParam(r, "operationID"), request.Revision, request.Command)
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Queue command accepted", op, http.StatusAccepted))
}

func (h *Handler) GetQueueDrainHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	c, ok := h.queueController(w, r)
	if !ok {
		return
	}
	history, err := c.Repository.History(r.Context(), c.Target.Scope, chi.URLParam(r, "operationID"))
	if err != nil {
		h.failQueueDrain(w, r, err)
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Queue operation history", history, http.StatusOK))
}

func decodeQueueCommand(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		var extra interface{}
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("unexpected trailing data")
		}
	}
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid queue operation request.", http.StatusBadRequest))
		return false
	}
	return true
}
func (h *Handler) failQueueDrain(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, drain.ErrStale):
		_ = render.Render(w, r, util.NewErrorResponse("Queue configuration or operation changed. Refresh before retrying.", http.StatusConflict))
	case errors.Is(err, drain.ErrConflict):
		_ = render.Render(w, r, util.NewErrorResponse("The command conflicts with the current queue operation.", http.StatusConflict))
	case errors.Is(err, drain.ErrEvidence):
		_ = render.Render(w, r, util.NewErrorResponse("Queue preflight has unresolved blockers. Refresh the review.", http.StatusConflict))
	case errors.Is(err, drain.ErrNotFound):
		_ = render.Render(w, r, util.NewErrorResponse("Queue operation not found.", http.StatusNotFound))
	default:
		h.A.Logger.ErrorContext(r.Context(), "queue operation failed", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Queue operation is unavailable.", http.StatusServiceUnavailable))
	}
}
