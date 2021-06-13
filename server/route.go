package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
)

func buildRoutes(app *applicationHandler) http.Handler {

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(writeRequestIDHeader)
	router.Use(middleware.AllowContentType("application/json"))
	router.Use(jsonResponse)

	router.Route("/v1", func(r chi.Router) {
		r.Use(requireAuth(app.orgRepo))

		r.Route("/apps", func(appRouter chi.Router) {

			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireAppOwnership(app.appRepo))

				appSubRouter.Get("/", app.GetApp)
				appSubRouter.Post("/{id}/message", nil)
			})
		})
	})

	return router
}

func New(cfg config.Configuration, appRepo hookcamp.ApplicationRepository,
	orgRepo hookcamp.OrganisationRepository) *http.Server {

	app := newApplicationHandler(appRepo, orgRepo)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	return srv
}
