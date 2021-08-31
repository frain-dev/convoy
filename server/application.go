package server

import (
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/hookcamp/hookcamp/server/models"
	"net/http"

	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type applicationHandler struct {
	appRepo hookcamp.ApplicationRepository
	orgRepo hookcamp.OrganisationRepository
	msgRepo hookcamp.MessageRepository
}

func newApplicationHandler(msgRepo hookcamp.MessageRepository, appRepo hookcamp.ApplicationRepository, orgRepo hookcamp.OrganisationRepository) *applicationHandler {

	return &applicationHandler{
		msgRepo: msgRepo,
		appRepo: appRepo,
		orgRepo: orgRepo,
	}
}

type organisationResponse struct {
	Organisation hookcamp.Organisation `json:"organisation"`
	Response
}

type organisationsResponse struct {
	Organisations []*hookcamp.Organisation `json:"organisations"`
	Response
}

type applicationsPagedResponse struct {
	Applications   []hookcamp.Application `json:"data"`
	PaginationData *pager.PaginationData  `json:"pagination"`
	Response
}

type dashboardSummaryResponse struct {
	DashboardSummary models.DashboardSummary `json:"dashboard"`
	Response
}

type applicationResponse struct {
	Application hookcamp.Application `json:"application"`
	Response
}

type messageResponse struct {
	Message hookcamp.Message `json:"message"`
	Response
}

type messagesResponse struct {
	Messages []hookcamp.Message `json:"messages"`
	Response
}
type messagesPagedResponse struct {
	Messages       []hookcamp.Message    `json:"messages"`
	PaginationData *pager.PaginationData `json:"pagination"`
	Response
}

type applicationsResponse struct {
	Applications []hookcamp.Application `json:"applications"`
	Response
}

type applicationEndpointResponse struct {
	Endpoint hookcamp.Endpoint `json:"endpoint"`
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

func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationResponse{
		Response: Response{
			StatusCode: http.StatusCreated,
		},
		Application: *getApplicationFromContext(r.Context()),
	})
}

func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationResponse{
		Response: Response{
			StatusCode: http.StatusAccepted,
		},
		Application: *getApplicationFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationsResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Applications: *getApplicationsFromContext(r.Context()),
	})
}

func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationEndpointResponse{
		Response: Response{
			StatusCode: http.StatusCreated,
		},
		Endpoint: *getApplicationEndpointFromContext(r.Context()),
	})
}

func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationEndpointResponse{
		Response: Response{
			StatusCode: http.StatusAccepted,
		},
		Endpoint: *getApplicationEndpointFromContext(r.Context()),
	})
}

func (a *applicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, organisationResponse{
		Response: Response{
			StatusCode: http.StatusCreated,
		},
		Organisation: *getOrganisationFromContext(r.Context()),
	})
}

func (a *applicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, organisationResponse{
		Response: Response{
			StatusCode: http.StatusAccepted,
		},
		Organisation: *getOrganisationFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, organisationResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Organisation: *getOrganisationFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetOrganisations(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, organisationsResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Organisations: getOrganisationsFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, dashboardSummaryResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		DashboardSummary: *getDashboardSummaryFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, applicationsPagedResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Applications:   *getApplicationsFromContext(r.Context()),
		PaginationData: getPaginationDataFromContext(r.Context()),
	})
}

func (a *applicationHandler) CreateAppMessage(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, messageResponse{
		Response: Response{
			StatusCode: http.StatusCreated,
		},
		Message: *getMessageFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetAppMessage(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, messageResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Message: *getMessageFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetAppMessages(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, messagesResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Messages: *getMessagesFromContext(r.Context()),
	})
}

func (a *applicationHandler) GetAppMessagesPaged(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, messagesPagedResponse{
		Response: Response{
			StatusCode: http.StatusOK,
		},
		Messages:       *getMessagesFromContext(r.Context()),
		PaginationData: getPaginationDataFromContext(r.Context()),
	})
}
