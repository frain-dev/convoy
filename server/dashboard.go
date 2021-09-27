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
	Database  config.DatabaseConfiguration  `json:"database"`
	Queue     config.QueueConfiguration     `json:"queue"`
	Server    config.ServerConfiguration    `json:"server"`
	Strategy  config.StrategyConfiguration  `json:"strategy"`
	Signature config.SignatureConfiguration `json:"signature"`
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
				Database:  cfg.Database,
				Queue:     cfg.Queue,
				Server:    cfg.Server,
				Strategy:  cfg.Strategy,
				Signature: cfg.Signature,
			}

			r = r.WithContext(setConfigInContext(r.Context(), &viewableConfig))
			next.ServeHTTP(w, r)
		})
	}
}
