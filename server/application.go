package server

import (
	"net/http"

	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/cache"
	limiter "github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/logger"

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
	appService        *services.AppService
	eventService      *services.EventService
	groupService      *services.GroupService
	securityService   *services.SecurityService
	appRepo           datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	groupRepo         datastore.GroupRepository
	apiKeyRepo        datastore.APIKeyRepository
	eventQueue        queue.Queuer
	logger            logger.Logger
	tracer            tracer.Tracer
	cache             cache.Cache
	limiter           limiter.RateLimiter
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

func newApplicationHandler(
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	groupRepo datastore.GroupRepository,
	apiKeyRepo datastore.APIKeyRepository,
	eventQueue queue.Queuer,
	logger logger.Logger,
	tracer tracer.Tracer,
	cache cache.Cache,
	limiter limiter.RateLimiter) *applicationHandler {
	as := services.NewAppService(appRepo, eventRepo, eventDeliveryRepo, eventQueue)
	es := services.NewEventService(appRepo, eventRepo, eventDeliveryRepo, eventQueue)
	gs := services.NewGroupService(appRepo, groupRepo, eventRepo, eventDeliveryRepo, limiter)
	ss := services.NewSecurityService(groupRepo, apiKeyRepo)

	return &applicationHandler{
		appService:        as,
		eventService:      es,
		groupService:      gs,
		securityService:   ss,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		apiKeyRepo:        apiKeyRepo,
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventQueue:        eventQueue,
		logger:            logger,
		tracer:            tracer,
		cache:             cache,
		limiter:           limiter,
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

	group := getGroupFromContext(r.Context())

	app, err := a.appService.CreateApp(r.Context(), &newApp, appName, group)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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
	var appUpdate models.UpdateApplication
	err := util.ReadJSON(r, &appUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := getApplicationFromContext(r.Context())

	err = a.appService.UpdateApplication(r.Context(), &appUpdate, app)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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
	err := a.appService.DeleteApplication(r.Context(), app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	app := getApplicationFromContext(r.Context())

	endpoint, err := a.appService.CreateAppEndpoint(r.Context(), e, app)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	endpoint, err := a.appService.UpdateAppEndpoint(r.Context(), e, endPointId, app)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	err := a.appService.DeleteAppEndpoint(r.Context(), e, app)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("App endpoint deleted successfully", nil, http.StatusOK))
}

func (a *applicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Apps fetched successfully",
		pagedResponse{Content: *getApplicationsFromContext(r.Context()),
			Pagination: getPaginationDataFromContext(r.Context())}, http.StatusOK))
}
