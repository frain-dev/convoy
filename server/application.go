package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	mongopagination "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type applicationHandler struct {
	appRepo           convoy.ApplicationRepository
	eventRepo         convoy.EventRepository
	eventDeliveryRepo convoy.EventDeliveryRepository
	groupRepo         convoy.GroupRepository
	eventQueue        queue.Queuer
}

type pagedResponse struct {
	Content    interface{}                     `json:"content,omitempty"`
	Pagination *mongopagination.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(eventRepo convoy.EventRepository,
	eventDeliveryRepo convoy.EventDeliveryRepository,
	appRepo convoy.ApplicationRepository,
	groupRepo convoy.GroupRepository,
	eventQueue queue.Queuer) *applicationHandler {

	return &applicationHandler{
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventQueue:        eventQueue,
	}
}

// GetApp
// @Summary Get an application
// @Description This endpoint fetches an application by it's id
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse{data=convoy.Application}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID} [get]
func (a *applicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App fetched successfully",
		*getApplicationFromContext(r.Context()), http.StatusOK))
}

// GetApps
// @Summary Get all applications
// @Description This fetches all applications
// @Tags Application
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]convoy.Application}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications [get]
func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())

	apps, paginationData, err := a.appRepo.LoadApplicationsPaged(r.Context(), group.UID, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching apps", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: &apps, Pagination: &paginationData}, http.StatusOK))
}

// CreateApp
// @Summary Create an application
// @Description This endpoint creates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param application body models.Application true "Application Details"
// @Success 200 {object} serverResponse{data=convoy.Application}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications [post]
func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	var newApp models.Application
	err := json.NewDecoder(r.Body).Decode(&newApp)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	appName := newApp.AppName
	if util.IsStringEmpty(appName) {
		_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
		return
	}

	group := getGroupFromContext(r.Context())

	uid := uuid.New().String()
	app := &convoy.Application{
		UID:            uid,
		GroupID:        group.UID,
		Title:          appName,
		SupportEmail:   newApp.SupportEmail,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		Endpoints:      []convoy.Endpoint{},
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.appRepo.CreateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App created successfully", app, http.StatusCreated))
}

// UpdateApp
// @Summary Update an application
// @Description This endpoint updates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param application body models.Application true "Application Details"
// @Success 200 {object} serverResponse{data=convoy.Application}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID} [put]
func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var appUpdate models.Application
	err := json.NewDecoder(r.Body).Decode(&appUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	appName := appUpdate.AppName
	if util.IsStringEmpty(appName) {
		_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
		return
	}

	app := getApplicationFromContext(r.Context())

	app.Title = appName
	if !util.IsStringEmpty(appUpdate.SupportEmail) {
		app.SupportEmail = appUpdate.SupportEmail
	}

	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App updated successfully", app, http.StatusAccepted))
}

// DeleteApp
// @Summary Delete app
// @Description This endpoint deletes an app
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID} [delete]
func (a *applicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())
	err := a.appRepo.DeleteApplication(r.Context(), app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App deleted successfully", nil, http.StatusOK))
}

// CreateAppEndpoint
// @Summary Create an application endpoint
// @Description This endpoint creates an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} serverResponse{data=convoy.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints [post]
func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r.Body)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	appID := chi.URLParam(r, "appID")
	app, err := a.appRepo.FindApplicationByID(r.Context(), appID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, convoy.ErrApplicationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	// Events being nil means it wasn't passed at all, which automatically
	// translates into a accept all scenario. This is quite different from
	// an empty array which signifies a blacklist all events -- no events
	// will be sent to such endpoints.
	if e.Events == nil {
		e.Events = []string{"*"}
	}

	endpoint := &convoy.Endpoint{
		UID:            uuid.New().String(),
		TargetURL:      e.URL,
		Description:    e.Description,
		Events:         e.Events,
		Secret:         e.Secret,
		Status:         convoy.ActiveEndpointStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	if util.IsStringEmpty(e.Secret) {
		endpoint.Secret, err = util.GenerateSecret()
		if err != nil {
			_ = render.Render(w, r, newErrorResponse(fmt.Sprintf("could not generate secret...%v", err.Error()), http.StatusInternalServerError))
			return
		}
	}

	app.Endpoints = append(app.Endpoints, *endpoint)

	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while adding app endpoint", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App endpoint created successfully", endpoint, http.StatusCreated))
}

// GetAppEndpoint
// @Summary Get application endpoint
// @Description This endpoint fetches an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} serverResponse{data=convoy.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints/{endpointID} [get]
func (a *applicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App endpoint fetched successfully",
		*getApplicationEndpointFromContext(r.Context()), http.StatusOK))
}

// GetAppEndpoints
// @Summary Get application endpoints
// @Description This endpoint fetches an application's endpoints
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse{data=[]convoy.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints [get]
func (a *applicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())

	app.Endpoints = filterDeletedEndpoints(app.Endpoints)
	_ = render.Render(w, r, newServerResponse("App endpoints fetched successfully", app.Endpoints, http.StatusOK))
}

// UpdateAppEndpoint
// @Summary Update an application endpoint
// @Description This endpoint updates an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} serverResponse{data=convoy.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints/{endpointID} [put]
func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r.Body)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := getApplicationFromContext(r.Context())
	endPointId := chi.URLParam(r, "endpointID")

	endpoints, endpoint, err := updateEndpointIfFound(&app.Endpoints, endPointId, e)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app.Endpoints = *endpoints
	err = a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating app endpoints", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Apps endpoint updated successfully", endpoint, http.StatusAccepted))
}

// DeleteAppEndpoint
// @Summary Delete application endpoint
// @Description This endpoint deletes an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints/{endpointID} [delete]
func (a *applicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {
	app := getApplicationFromContext(r.Context())
	e := getApplicationEndpointFromContext(r.Context())

	for i, endpoint := range app.Endpoints {
		if endpoint.UID == e.UID && endpoint.DeletedAt == 0 {
			app.Endpoints = append(app.Endpoints[:i], app.Endpoints[i+1:]...)
			break
		}
	}

	err := a.appRepo.UpdateApplication(r.Context(), app)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app endpoint", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App endpoint deleted successfully", nil, http.StatusOK))
}

func (a *applicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: *getApplicationsFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}

func updateEndpointIfFound(endpoints *[]convoy.Endpoint, id string, e models.Endpoint) (*[]convoy.Endpoint, *convoy.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description
			endpoint.Events = e.Events
			endpoint.Status = convoy.ActiveEndpointStatus
			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, convoy.ErrEndpointNotFound
}
