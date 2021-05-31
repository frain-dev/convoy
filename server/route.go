package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func New() *http.Server {

	router := chi.NewRouter()

	router.Route("/v1", func(r chi.Router) {

		r.Route("/apps", func(appRouter chi.Router) {

			appRouter.Get("/", nil)
			appRouter.Get("/{id}", nil)
			appRouter.Post("{id}/message", nil)
		})

	})

	return nil
}
