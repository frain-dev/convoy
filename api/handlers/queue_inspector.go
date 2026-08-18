package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// GetQueueHistory returns one queue's daily processed and failed counts.
func (h *Handler) GetQueueHistory(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	queueName, ok := queueNameParam(w, r)
	if !ok {
		return
	}
	days, ok := intQuery(w, r, "days", queue.DefaultHistoryDays)
	if !ok {
		return
	}

	history, err := inspector.History(r.Context(), queueName, days)
	if err != nil {
		h.failQueueRequest(w, r, "failed to load queue history", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("queue history fetched successfully", history, http.StatusOK))
}

// GetQueueSchedulerEntries returns the periodic tasks registered on this instance.
func (h *Handler) GetQueueSchedulerEntries(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	entries, err := inspector.SchedulerEntries(r.Context())
	if err != nil {
		h.failQueueRequest(w, r, "failed to load scheduler entries", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("scheduler entries fetched successfully", entries, http.StatusOK))
}

// GetQueueTasks returns one page of a queue's tasks in the requested status.
func (h *Handler) GetQueueTasks(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	page, ok := intQuery(w, r, "page", 1)
	if !ok {
		return
	}
	perPage, ok := intQuery(w, r, "perPage", queue.TasksPerPage)
	if !ok {
		return
	}

	queueName, ok := queueNameParam(w, r)
	if !ok {
		return
	}

	tasks, err := inspector.Tasks(r.Context(), queue.TaskFilter{
		Queue:    queueName,
		Status:   r.URL.Query().Get("status"),
		Page:     page,
		PageSize: perPage,
		// An id pasted from a log carries whitespace often enough that
		// rejecting it would only teach the operator to trim by hand.
		Search: strings.TrimSpace(r.URL.Query().Get("search")),
	})
	if err != nil {
		h.failQueueRequest(w, r, "failed to load queue tasks", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("queue tasks fetched successfully", tasks, http.StatusOK))
}

// RetryQueueTask returns one archived task to the queue.
func (h *Handler) RetryQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, queue.ActionRetry, "queue task retried")
}

// RunQueueTask pulls one waiting task forward to now.
func (h *Handler) RunQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, queue.ActionRun, "queue task scheduled to run now")
}

// ArchiveQueueTask takes one task out of the queue.
func (h *Handler) ArchiveQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, queue.ActionArchive, "queue task archived")
}

// DeleteQueueTask drops one task for good.
func (h *Handler) DeleteQueueTask(w http.ResponseWriter, r *http.Request) {
	h.runQueueTaskAction(w, r, queue.ActionDelete, "queue task deleted")
}

func (h *Handler) runQueueTaskAction(w http.ResponseWriter, r *http.Request, action, success string) {
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

	var err error
	switch action {
	case queue.ActionRetry:
		err = inspector.RetryTask(r.Context(), queueName, taskID)
	case queue.ActionRun:
		err = inspector.RunTask(r.Context(), queueName, taskID)
	case queue.ActionArchive:
		err = inspector.ArchiveTask(r.Context(), queueName, taskID)
	case queue.ActionDelete:
		err = inspector.DeleteTask(r.Context(), queueName, taskID)
	default:
		err = queue.ErrUnknownTaskAction
	}
	if err != nil {
		h.failQueueRequest(w, r, "failed to run queue task action", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(success, nil, http.StatusOK))
}

// BulkQueueTaskAction runs one action over the tasks an operator selected. It
// answers 200 with per-id outcomes even when some ids were refused, because a
// partial result is the accurate answer: some rows did move, and the caller
// needs to know which. Only a failure to reach the broker at all is an error.
func (h *Handler) BulkQueueTaskAction(w http.ResponseWriter, r *http.Request) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	queueName, ok := queueNameParam(w, r)
	if !ok {
		return
	}

	var body struct {
		Action  string   `json:"action"`
		TaskIDs []string `json:"task_ids"`
	}
	// One selection is a page of ids at most, so a body far larger than that is
	// not a selection this endpoint can serve and is refused before it is read.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBulkActionBody)).Decode(&body); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("invalid request body", http.StatusBadRequest))
		return
	}

	result, err := inspector.BulkAction(r.Context(), queueName, body.Action, body.TaskIDs)
	if err != nil {
		h.failQueueRequest(w, r, "failed to run bulk queue task action", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("bulk action completed", result, http.StatusOK))
}

// PauseQueue stops workers claiming from a queue.
func (h *Handler) PauseQueue(w http.ResponseWriter, r *http.Request) {
	h.runQueueStateChange(w, r, true)
}

// UnpauseQueue lets workers claim from a queue again.
func (h *Handler) UnpauseQueue(w http.ResponseWriter, r *http.Request) {
	h.runQueueStateChange(w, r, false)
}

func (h *Handler) runQueueStateChange(w http.ResponseWriter, r *http.Request, pause bool) {
	inspector, ok := h.queueInspector(w, r)
	if !ok {
		return
	}

	queueName, ok := queueNameParam(w, r)
	if !ok {
		return
	}

	var (
		err     error
		success string
	)
	if pause {
		err, success = inspector.PauseQueue(r.Context(), queueName), "queue paused"
	} else {
		err, success = inspector.UnpauseQueue(r.Context(), queueName), "queue resumed"
	}
	if err != nil {
		h.failQueueRequest(w, r, "failed to change queue state", err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(success, nil, http.StatusOK))
}

// maxBulkActionBody bounds the bulk request body. MaxTasksPerPage ids of asynq
// or uuid length is well under this, so the slack is for formatting rather than
// for a larger selection, which RunBulkAction rejects anyway.
const maxBulkActionBody = 64 << 10

// queueNameParam reads the queue a request addresses. Every queue endpoint is
// scoped to one queue, and the providers disagree about what an empty name
// means to the broker, so it is rejected here and both answer the same way.
func queueNameParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := chi.URLParam(r, "queueName")
	if name == "" {
		_ = render.Render(w, r, util.NewErrorResponse("queue name is required", http.StatusBadRequest))
		return "", false
	}
	return name, true
}

// intQuery reads an optional numeric query parameter. An absent value takes the
// default; a present but unparseable one is rejected rather than silently
// defaulted, because a caller that sent a page meant to be on it.
func intQuery(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(name+" must be a number", http.StatusBadRequest))
		return 0, false
	}
	return n, true
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
	if !h.requireStrictInstanceAdmin(w, r) {
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
		errors.Is(err, queue.ErrUnknownTaskStatus),
		errors.Is(err, queue.ErrUnknownTaskAction),
		errors.Is(err, queue.ErrNoTaskIDs),
		errors.Is(err, queue.ErrTooManyTaskIDs):
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
