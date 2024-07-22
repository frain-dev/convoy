package handlers

import (
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/util"
)

// GetDeliveryAttempt
//
//	@Summary		Retrieve a delivery attempt
//	@Description	This endpoint fetches an app event delivery attempt
//	@Tags			Delivery Attempts
//	@Id				GetDeliveryAttempt
//	@Accept			json
//	@Produce		json
//	@Param			projectID			path		string	true	"Project ID"
//	@Param			eventDeliveryID		path		string	true	"event delivery id"
//	@Param			deliveryAttemptID	path		string	true	"delivery attempt id"
//	@Success		200					{object}	util.ServerResponse{data=datastore.DeliveryAttempt}
//	@Failure		400,401,404			{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts/{deliveryAttemptID} [get]
func (h *Handler) GetDeliveryAttempt(w http.ResponseWriter, r *http.Request) {
	deliveryAttemptID := chi.URLParam(r, "deliveryAttemptID")
	eventDelivery, err := h.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if len(eventDelivery.DeliveryAttempts) > 0 {
		deliveryAttempt, deliveryErr := findDeliveryAttempt(eventDelivery.DeliveryAttempts, deliveryAttemptID)
		if deliveryErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(deliveryErr))
			return
		}

		_ = render.Render(w, r, util.NewServerResponse("Event delivery attempt fetched successfully", deliveryAttempt, http.StatusOK))
		return
	}

	attemptsRepo := postgres.NewDeliveryAttemptRepo(h.A.DB)
	deliveryAttempt, err := attemptsRepo.FindDeliveryAttemptById(r.Context(), eventDelivery.UID, deliveryAttemptID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Event delivery attempt fetched successfully", deliveryAttempt, http.StatusOK))
}

// GetDeliveryAttempts
//
//	@Summary		List delivery attempts
//	@Description	This endpoint fetches an app message's delivery attempts
//	@Tags			Delivery Attempts
//	@Id				GetDeliveryAttempts
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			eventDeliveryID	path		string	true	"event delivery id"
//	@Success		200				{object}	util.ServerResponse{data=[]datastore.DeliveryAttempt}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts [get]
func (h *Handler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := h.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	attemptsRepo := postgres.NewDeliveryAttemptRepo(h.A.DB)
	attempts, err := attemptsRepo.FindDeliveryAttempts(r.Context(), eventDelivery.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	eventDelivery.DeliveryAttempts = append(eventDelivery.DeliveryAttempts, attempts...)

	_ = render.Render(w, r, util.NewServerResponse("Event delivery attempts fetched successfully", eventDelivery.DeliveryAttempts, http.StatusOK))
}

func findDeliveryAttempt(attempts []datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}
