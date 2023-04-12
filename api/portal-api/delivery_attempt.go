package portalapi

import (
	"net/http"

	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

func (a *PortalLinkHandler) GetDeliveryAttempt(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempt fetched successfully",
		*m.GetDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

func (a *PortalLinkHandler) GetDeliveryAttempts(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("App event delivery attempts fetched successfully",
		*m.GetDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}
