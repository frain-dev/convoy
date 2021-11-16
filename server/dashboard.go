package server

import (
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
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

			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Error("error while fetching config details")
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching config details", http.StatusInternalServerError))
				return
			}

			viewableConfig := ViewableConfiguration{
				Strategy:  cfg.Strategy,
				Signature: cfg.Signature,
			}

			r = r.WithContext(setConfigInContext(r.Context(), &viewableConfig))
			next.ServeHTTP(w, r)
		})
	}
}
