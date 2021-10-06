package server

import (
	"net/http"

	"github.com/go-chi/render"
)

func (a *applicationHandler) GetAppMessageDeliveryAttempt(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempt fetched successfully",
		*getDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAppMessageDeliveryAttempts(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempts fetched successfully",
		*getDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}
