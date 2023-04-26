package portalapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

func (a *PortalLinkHandler) GetDeliveryAttempt(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
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

func (a *PortalLinkHandler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
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
