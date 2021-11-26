package server

import (
	"net/http"

	"github.com/go-chi/render"
)

// GetDeliveryAttempt
// @Summary Get delivery attempt
// @Description This endpoint fetches an app event delivery attempt
// @Tags DeliveryAttempts
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Param eventDeliveryID path string true "event delivery id"
// @Param deliveryAttemptID path string true "delivery attempt id"
// @Success 200 {object} serverResponse{data=convoy.DeliveryAttempt}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/{eventDeliveryID}/deliveryattempts/{deliveryAttemptID} [get]
func (a *applicationHandler) GetDeliveryAttempt(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempt fetched successfully",
		*getDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

// GetDeliveryAttempts
// @Summary Get delivery attempts
// @Description This endpoint fetches an app message's delivery attempts
// @Tags DeliveryAttempts
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} serverResponse{data=[]convoy.DeliveryAttempt}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/{eventDeliveryID}/deliveryattempts [get]
func (a *applicationHandler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempts fetched successfully",
		*getDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}
