package server

import (
	"errors"
	"net/http"
	"time"

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
	response
	Application hookcamp.Application `json:"application"`
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

		render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	render.Render(w, r, applicationResponse{
		response: response{
			StatusCode: http.StatusOK,
			Timestamp:  time.Now().Unix(),
		},
		Application: *app,
	})
}
