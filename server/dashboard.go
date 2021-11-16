package server

import (
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
)

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

type ViewableConfiguration struct {
	Strategy  config.StrategyConfiguration  `json:"strategy"`
	Signature config.SignatureConfiguration `json:"signature"`
}

func (a *applicationHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Dashboard summary fetched successfully",
		*getDashboardSummaryFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Logged in successfully",
		getAuthLoginFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAllConfigDetails(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Config details fetched successfully",
		getConfigFromContext(r.Context()), http.StatusOK))
}

func fetchAllConfigDetails() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			g := getGroupFromContext(r.Context())

			viewableConfig := ViewableConfiguration{
				Strategy:  g.Config.Strategy,
				Signature: g.Config.Signature,
			}

			r = r.WithContext(setConfigInContext(r.Context(), &viewableConfig))
			next.ServeHTTP(w, r)
		})
	}
}
