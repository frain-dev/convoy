package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/logger"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

type applicationHandler struct {
	appRepo           datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	groupRepo         datastore.GroupRepository
	apiKeyRepo        datastore.APIKeyRepository
	eventQueue        queue.Queuer
	logger            logger.Logger
	tracer            tracer.Tracer
	cache             cache.Cache
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	groupRepo datastore.GroupRepository,
	apiKeyRepo datastore.APIKeyRepository,
	eventQueue queue.Queuer, logger logger.Logger, tracer tracer.Tracer, cache cache.Cache) *applicationHandler {

	return &applicationHandler{
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		apiKeyRepo:        apiKeyRepo,
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventQueue:        eventQueue,
		logger:            logger,
		tracer:            tracer,
		cache:             cache,
	}
}

// GetApp
// @Summary Get an application
// @Description This endpoint fetches an application by it's id
// @Tags Application
// @Accept  json
// @Produce  json
// @Param appID path string true "application id"
// @Success 200 {object} serverResponse{data=datastore.Application}
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
// @Param q query string false "app title"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Application}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications [get]
func (a *applicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())
	q := r.URL.Query().Get("q")

	apps, paginationData, err := a.appRepo.LoadApplicationsPaged(r.Context(), group.UID, q, pageable)
	if err != nil {
		print(err.Error())
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching apps. Error: "+err.Error(), http.StatusBadRequest))
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
// @Success 200 {object} serverResponse{data=datastore.Application}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications [post]
func (a *applicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {

	var newApp models.Application
	err := util.ReadJSON(r, &newApp)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	appName := newApp.AppName
	if err = util.Validate(newApp); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := getGroupFromContext(r.Context())

	uid := uuid.New().String()
	app := &datastore.Application{
		UID:            uid,
		GroupID:        group.UID,
		Title:          appName,
		SupportEmail:   newApp.SupportEmail,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		Endpoints:      []datastore.Endpoint{},
		DocumentStatus: datastore.ActiveDocumentStatus,
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
// @Success 200 {object} serverResponse{data=datastore.Application}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID} [put]
func (a *applicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var appUpdate models.Application
	err := util.ReadJSON(r, &appUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	appName := appUpdate.AppName
	if err = util.Validate(appUpdate); err != nil {
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
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating app", http.StatusBadRequest))
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
// @Success 200 {object} serverResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints [post]
func (a *applicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	appID := chi.URLParam(r, "appID")
	app, err := a.appRepo.FindApplicationByID(r.Context(), appID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusBadRequest

		if errors.Is(err, datastore.ErrApplicationNotFound) {
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

	endpoint := &datastore.Endpoint{
		UID:            uuid.New().String(),
		TargetURL:      e.URL,
		Description:    e.Description,
		Events:         e.Events,
		Secret:         e.Secret,
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
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
// @Success 200 {object} serverResponse{data=datastore.Endpoint}
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
// @Success 200 {object} serverResponse{data=[]datastore.Endpoint}
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
// @Success 200 {object} serverResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /applications/{appID}/endpoints/{endpointID} [put]
func (a *applicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := parseEndpointFromBody(r)
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

func updateEndpointIfFound(endpoints *[]datastore.Endpoint, id string, e models.Endpoint) (*[]datastore.Endpoint, *datastore.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description
			endpoint.Events = e.Events
			endpoint.Status = datastore.ActiveEndpointStatus
			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, datastore.ErrEndpointNotFound
}
