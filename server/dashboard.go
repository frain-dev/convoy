package server

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
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

func fetchAllConfigDetails() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cfg, err := config.Get()
			if err != nil {
				log.Errorln("error while fetching config details - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching config details", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setConfigInContext(r.Context(), &cfg))
			next.ServeHTTP(w, r)
		})
	}
}
