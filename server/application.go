package server

import (
	"net/http"

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

	_ = render.Render(w, r, applicationResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Application: *getApplicationFromContext(r.Context()),
	})
}
