package handlers

import (
	"net/http"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (h *Handler) TriggerRetentionPolicyJob(w http.ResponseWriter, r *http.Request) {
	if !h.A.Licenser.AdvancedRetentionPolicy() {
		_ = render.Render(w, r, util.NewErrorResponse("your instance does not have access to this feature, please upgrade to access it", http.StatusBadRequest))
		return
	}

	job := &queue.Job{
		ID:    ulid.Make().String(),
		Delay: 0,
	}

	err := h.A.Queue.Write(convoy.RetentionPolicies, convoy.ScheduleQueue, job)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Errorf("failed to trigger retention policy job")
	}

	_ = render.Render(w, r, util.NewServerResponse("retention policy job triggered successfully", 200, http.StatusCreated))
	return
}
