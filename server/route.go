package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hookcamp/hookcamp"
)

func New(appRepo hookcamp.ApplicationRepository,
	orgRepo hookcamp.OrganisationRepository) *http.Server {

	app := newApplicationHandler(appRepo, orgRepo)

	router := chi.NewRouter()

	router.Route("/v1", func(r chi.Router) {

		r.Route("/apps", func(appRouter chi.Router) {

			appRouter.Get("/{id}", app.GetApp)
			appRouter.Post("{id}/message", nil)
		})

	})

	return nil
}
