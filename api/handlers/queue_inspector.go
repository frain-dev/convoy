package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

// GetQueueStats returns per-queue depth for the running broker.
func (h *Handler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	stats, err := inspector.Stats(r.Context())
	if err != nil {
		h.failQueueRequest(w, r, "failed to load queue stats", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("queue stats fetched successfully", stats, http.StatusOK))
}

// GetQueueTasks returns one page of a queue's tasks in the requested status.
func (h *Handler) GetQueueTasks(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("page must be a number", http.StatusBadRequest))
			return
		}
		page = n
	}

	tasks, err := inspector.Tasks(r.Context(), queue.TaskFilter{
		Queue:  chi.URLParam(r, "queueName"),
		Status: r.URL.Query().Get("status"),
		Page:   page,
	})
	if err != nil {
		h.failQueueRequest(w, r, "failed to load queue tasks", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("queue tasks fetched successfully", tasks, http.StatusOK))
}

// RetryQueueTask returns one task to the queue.
func (h *Handler) RetryQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, func(i queue.Inspector, queueName, taskID string) error {
		return i.RetryTask(r.Context(), queueName, taskID)
	}, "queue task retried")
}

// ArchiveQueueTask takes one task out of the queue.
func (h *Handler) ArchiveQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, func(i queue.Inspector, queueName, taskID string) error {
		return i.ArchiveTask(r.Context(), queueName, taskID)
	}, "queue task archived")
}

func (h *Handler) runQueueTaskAction(w http.ResponseWriter, r *http.Request, action func(queue.Inspector, string, string) error, success string) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	queueName := chi.URLParam(r, "queueName")
	taskID := chi.URLParam(r, "taskID")
	if queueName == "" || taskID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("queue name and task id are required", http.StatusBadRequest))
		return
	}

	if err := action(inspector, queueName, taskID); err != nil {
		h.failQueueRequest(w, r, "failed to run queue task action", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(success, nil, http.StatusOK))
}

// queueInspector is the one gate every queue endpoint passes through. The
// authorization lives here rather than on the route group because the sibling
// /ui/admin routes carry their own checks, so a group middleware is easy to
// lose in a later re-registration; a handler cannot reach the broker without
// coming through this. Queue contents are instance-wide, so a page of tasks
// carries the last error text of every org's deliveries.
//
// It answers 501 rather than panicking when a deployment has no inspector
// wired. Every caller must stop on false.
func (h *Handler) queueInspector(w http.ResponseWriter, r *http.Request) (queue.Inspector, bool) {
	if !h.requireInstanceAdmin(w, r) {
		return nil, false
	}
	if h.A.QueueInspector == nil {
		_ = render.Render(w, r, util.NewErrorResponse("queue monitoring is unavailable on this queue provider", http.StatusNotImplemented))
		return nil, false
	}
	return h.A.QueueInspector, true
}

// failQueueRequest maps the errors the inspector classifies. Anything it does
// not classify is a broker or database failure: it is logged and answered with
// the generic message, because those errors carry connection strings and row
// detail that must not reach the caller.
func (h *Handler) failQueueRequest(w http.ResponseWriter, r *http.Request, generic string, err error) {
	switch {
	case errors.Is(err, queue.ErrQueueRequired),
		errors.Is(err, queue.ErrInvalidPage),
		errors.Is(err, queue.ErrUnknownTaskStatus):
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
	case errors.Is(err, queue.ErrQueueNotFound),
		errors.Is(err, queue.ErrTaskNotFound):
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
	case errors.Is(err, queue.ErrCronTaskImmutable),
		errors.Is(err, queue.ErrTaskStatusConflict):
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
	default:
		h.A.Logger.ErrorContext(r.Context(), generic, "error", err)
		_ = render.Render(w, r, util.NewErrorResponse(generic, http.StatusInternalServerError))
	}
}
