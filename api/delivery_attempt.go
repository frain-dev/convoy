package api

import (
	"net/http"

	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

// GetDeliveryAttempt
// @Summary Get delivery attempt
// @Description This endpoint fetches an app event delivery attempt
// @Tags Delivery Attempts
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
	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempt fetched successfully",
		*m.GetDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

// GetDeliveryAttempts
// @Summary Get delivery attempts
// @Description This endpoint fetches an app message's delivery attempts
// @Tags Delivery Attempts
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.DeliveryAttempt}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts [get]
func (a *ApplicationHandler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempts fetched successfully",
		*m.GetDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}
