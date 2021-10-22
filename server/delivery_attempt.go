package server

import (
	"net/http"

	"github.com/go-chi/render"
)

// GetAppMessageDeliveryAttempt
// @Summary Get app message delivery attempt
// @Description This endpoint fetches an app message delivery attempt
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Param deliveryAttemptID path string true "delivery attempt id"
// @Success 200 {object} serverResponse{data=convoy.MessageAttempt}
// @Failure 400,401,500 {object} serverResponse{data=Empty}
// @Security ApiKeyAuth
// @Router /events/{eventID}/deliveryattempts/{deliveryAttemptID} [get]
func (a *applicationHandler) GetAppMessageDeliveryAttempt(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempt fetched successfully",
		*getDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

// GetAppMessageDeliveryAttempts
// @Summary Get app message delivery attempts
// @Description This endpoint fetches an app message's delivery attempts
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse{data=[]convoy.MessageAttempt}
// @Failure 400,401,500 {object} serverResponse{data=Empty}
// @Security ApiKeyAuth
// @Router /events/{eventID}/deliveryattempts [get]
func (a *applicationHandler) GetAppMessageDeliveryAttempts(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempts fetched successfully",
		*getDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}
