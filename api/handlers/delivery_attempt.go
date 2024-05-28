package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
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
	eventDelivery, err := h.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	deliveryAttemptID := chi.URLParam(r, "deliveryAttemptID")
	attempts := (*[]datastore.DeliveryAttempt)(&eventDelivery.DeliveryAttempts)
	deliveryAttempt, err := findDeliveryAttempt(attempts, deliveryAttemptID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempt fetched successfully",
		deliveryAttempt, http.StatusOK))
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

	attempts := (*[]datastore.DeliveryAttempt)(&eventDelivery.DeliveryAttempts)
	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempts fetched successfully",
		attempts, http.StatusOK))
}

func findDeliveryAttempt(attempts *[]datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}
