package dashboard

import (
	"net/http"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func (a *DashboardHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Logged in successfully",
		middleware.GetAuthLoginFromContext(r.Context()), http.StatusOK))
}
