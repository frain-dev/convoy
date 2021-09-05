package server

import (
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

func fetchAuthConfig() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cfg, err := config.Get()
			if err != nil {
				log.Errorln("error while fetching auth config - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching authorization details", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setAuthConfigInContext(r.Context(), &cfg.Auth))
			next.ServeHTTP(w, r)
		})
	}
}
