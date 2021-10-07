package server

import (
	"net/http"

	mongopagination "github.com/gobeam/mongo-go-pagination"

	"github.com/frain-dev/convoy"
	"github.com/go-chi/render"
)

type applicationHandler struct {
	appRepo convoy.ApplicationRepository
	orgRepo convoy.OrganisationRepository
	msgRepo convoy.MessageRepository
}

type pagedResponse struct {
	Content    interface{}                     `json:"content,omitempty"`
	Pagination *mongopagination.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(msgRepo convoy.MessageRepository, appRepo convoy.ApplicationRepository, orgRepo convoy.OrganisationRepository) *applicationHandler {

	return &applicationHandler{
		msgRepo: msgRepo,
		appRepo: appRepo,
		orgRepo: orgRepo,
	}
}

// @Summary Get an application
// @Description This endpoint fetches an application by it's id
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse // TODO(daniel): should this be?, serverResponse's data field is an interface, this makes the generated doc vauge
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID} [get]
func (a *applicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App fetched successfully",
		*getApplicationFromContext(r.Context()), http.StatusOK))
}

// @Summary Create an application
// @Description This endpoint creates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param application body models.Application true "Application Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID} [post]
func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App created successfully",
		*getApplicationFromContext(r.Context()), http.StatusCreated))
}

// @Summary Update an application
// @Description This endpoint updates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param application body models.Application true "Application Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID} [post]
func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App updated successfully",
		*getApplicationFromContext(r.Context()), http.StatusAccepted))
}

// @Summary Get all applications
// @Description This fetches all application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps [get]
func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: *getApplicationsFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}

// @Summary Create an application endpoint
// @Description This endpoint creates an application endpoint
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/endpoints [post]
func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint created successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusCreated))
}

// @Summary Update an application endpoint
// @Description This endpoint updates an application endpoint
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/endpoints/{endpointID} [put]
func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps endpoint updated successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusAccepted))
}

// @Summary Get application endpoint
// @Description This endpoint fetches an application endpoint
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/endpoints/{endpointID} [get]
func (a *applicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint fetched successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusOK))
}

// @Summary Delete application endpoint
// @Description This endpoint deletes an application endpoint
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/endpoints/{endpointID} [delete]
func (a *applicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint deleted successfully",
		nil, http.StatusOK))
}

// @Summary Get application endpoints
// @Description This endpoint deletes an application's endpoints
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/endpoints [get]
func (a *applicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoints fetched successfully",
		*getApplicationEndpointsFromContext(r.Context()), http.StatusOK))
}

// @Summary Create an organisation
// @Description This endpoint creates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations [post]
func (a *applicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation created successfully",
		*getOrganisationFromContext(r.Context()), http.StatusCreated))
}

// @Summary Update an organisation
// @Description This endpoint updates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [put]
func (a *applicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation updated successfully",
		*getOrganisationFromContext(r.Context()), http.StatusAccepted))
}

// @Summary Get an organisation
// @Description This endpoint fetches an organisation by it's id
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [get]
func (a *applicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation fetched successfully",
		*getOrganisationFromContext(r.Context()), http.StatusOK))
}

// @Summary Get organisations
// @Description This endpoint fetches organisations
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [get]
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

// @Summary Create app message
// @Description This endpoint creates an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param message body models.Message true "Message Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events [post]
func (a *applicationHandler) CreateAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event created successfully",
		*getMessageFromContext(r.Context()), http.StatusCreated))
}

// @Summary Get app message
// @Description This endpoint fetches an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID} [get]
func (a *applicationHandler) GetAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event fetched successfully",
		*getMessageFromContext(r.Context()), http.StatusOK))
}

// @Summary Resend an app message
// @Description This endpoint resends an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventIDD path string true "event id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID}/resend [put]
func (a *applicationHandler) ResendAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event processed for retry successfully",
		*getMessageFromContext(r.Context()), http.StatusOK))
}

// @Summary Get app messages with pagination
// @Description This endpoint fetches app messages with pagination
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID}/events [get]
func (a *applicationHandler) GetAppMessagesPaged(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App events fetched successfully",
		pagedResponse{Content: *getMessagesFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}

// @Summary Delete app
// @Description This endpoint deletes an app
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /apps/{appID} [delete]
func (a *applicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App deleted successfully",
		nil, http.StatusOK))
}

// @Summary Get app message delivery attempt
// @Description This endpoint fetches an app message delivery attempt
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Param deliveryAttemptID path string true "delivery attempt id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID}/deliveryattempts/{deliveryAttemptID} [get]
func (a *applicationHandler) GetAppMessageDeliveryAttempt(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempt fetched successfully",
		*getDeliveryAttemptFromContext(r.Context()), http.StatusOK))
}

// @Summary Get app message delivery attempts
// @Description This endpoint fetches an app message's delivery attempts
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID}/deliveryattempts [get]
func (a *applicationHandler) GetAppMessageDeliveryAttempts(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event delivery attempts fetched successfully",
		*getDeliveryAttemptsFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAuthDetails(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Auth details fetched successfully",
		getAuthConfigFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Logged in successfully",
		getAuthLoginFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAllConfigDetails(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Config details fetched successfully",
		getConfigFromContext(r.Context()), http.StatusOK))
}
