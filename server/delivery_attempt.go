package server

import (
	"net/http"

	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

// GetDeliveryAttempt
// @Summary Get delivery attempt
// @Description This endpoint fetches an app event delivery attempt
// @Tags DeliveryAttempts
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param eventDeliveryID path string true "event delivery id"
// @Param deliveryAttemptID path string true "delivery attempt id"
// @Success 200 {object} util.ServerResponse{data=datastore.DeliveryAttempt}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts/{deliveryAttemptID} [get]
func (a *ApplicationHandler) GetDeliveryAttempt(w http.ResponseWriter, r *http.Request) {
	eventService := createEventService(a)
	eventDelivery, err := eventService.GetEventDeliveryByID(r.Context(), m.GetEventDeliveryID(r))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	at, err := eventDelivery.FindDeliveryAttempt(m.GetDeliveryAttemptID(r))
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempt fetched successfully",
		at, http.StatusOK))
}

// GetDeliveryAttempts
// @Summary Get delivery attempts
// @Description This endpoint fetches an app message's delivery attempts
// @Tags DeliveryAttempts
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.DeliveryAttempt}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts [get]
func (a *ApplicationHandler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	eventService := createEventService(a)
	eventDelivery, err := eventService.GetEventDeliveryByID(r.Context(), m.GetEventDeliveryID(r))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempts fetched successfully",
		eventDelivery.DeliveryAttempts, http.StatusOK))
}
