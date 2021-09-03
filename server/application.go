package server

import (
	mongopagination "github.com/gobeam/mongo-go-pagination"
	"net/http"

	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type applicationHandler struct {
	appRepo hookcamp.ApplicationRepository
	orgRepo hookcamp.OrganisationRepository
	msgRepo hookcamp.MessageRepository
}

type pagedResponse struct {
	Content    interface{}                     `json:"content,omitempty"`
	Pagination *mongopagination.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(msgRepo hookcamp.MessageRepository, appRepo hookcamp.ApplicationRepository, orgRepo hookcamp.OrganisationRepository) *applicationHandler {

	return &applicationHandler{
		msgRepo: msgRepo,
		appRepo: appRepo,
		orgRepo: orgRepo,
	}
}

func (a *applicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App fetched successfully",
		*getApplicationFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App created successfully",
		*getApplicationFromContext(r.Context()), http.StatusCreated))
}

func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App updated successfully",
		*getApplicationFromContext(r.Context()), http.StatusAccepted))
}

func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		*getApplicationsFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint created successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusCreated))
}

func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps endpoint updated successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusAccepted))
}

func (a *applicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint fetched successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint deleted successfully",
		nil, http.StatusOK))
}

func (a *applicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoints fetched successfully",
		*getApplicationEndpointsFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation created successfully",
		*getOrganisationFromContext(r.Context()), http.StatusCreated))
}

func (a *applicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation updated successfully",
		*getOrganisationFromContext(r.Context()), http.StatusAccepted))
}

func (a *applicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation fetched successfully",
		*getOrganisationFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetOrganisations(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisations fetched successfully",
		getOrganisationsFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Dashboard summary fetched successfully",
		*getDashboardSummaryFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: *getApplicationsFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}

func (a *applicationHandler) CreateAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event created successfully",
		*getMessageFromContext(r.Context()), http.StatusCreated))
}

func (a *applicationHandler) GetAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event fetched successfully",
		*getMessageFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAppMessages(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App events fetched successfully",
		*getMessagesFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAppMessagesPaged(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App events fetched successfully",
		pagedResponse{Content: *getMessagesFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}

func (a *applicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App deleted successfully",
		nil, http.StatusOK))
}

func (a *applicationHandler) GetAppMessageDeliveryAttempt(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempt fetched successfully",
		*getDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAppMessageDeliveryAttempts(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempts fetched successfully",
		*getDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAuthDetails(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Auth details fetched successfully",
		getAuthConfigFromContext(r.Context()), http.StatusOK))
}
