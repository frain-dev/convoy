package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type applicationHandler struct {
	appRepo hookcamp.ApplicationRepository
	orgRepo hookcamp.OrganisationRepository
}

func newApplicationHandler(appRepo hookcamp.ApplicationRepository,
	orgRepo hookcamp.OrganisationRepository) *applicationHandler {

	return &applicationHandler{
		appRepo: appRepo,
		orgRepo: orgRepo,
	}
}

type applicationResponse struct {
	Application hookcamp.Application `json:"application"`
	Response
}

func (a *applicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {

	appID := chi.URLParam(r, "id")

	app, err := a.appRepo.FindApplicationByID(r.Context(), appID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, hookcamp.ErrApplicationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	_ = render.Render(w, r, applicationResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Application: *app,
	})
}
