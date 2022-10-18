package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

func createApplicationService(a *ApplicationHandler) *services.AppService {
	appRepo := mongo.NewApplicationRepo(a.A.Store)
	eventRepo := mongo.NewEventRepository(a.A.Store)
	eventDeliveryRepo := mongo.NewEventDeliveryRepository(a.A.Store)

	return services.NewAppService(
		appRepo, eventRepo, eventDeliveryRepo, a.A.Cache,
	)
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

// GetApp
// @Summary Get an application
// @Description This endpoint fetches an application by it's id
// @Tags Application
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Success 200 {object} util.ServerResponse{data=datastore.Application}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID} [get]
func (a *ApplicationHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("App fetched successfully",
		*m.GetApplicationFromContext(r.Context()), http.StatusOK))
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
// @Param projectID path string true "Project id"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Application}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications [get]
func (a *ApplicationHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())
	q := r.URL.Query().Get("q")
	appService := createApplicationService(a)

	apps, paginationData, err := appService.LoadApplicationsPaged(r.Context(), group.UID, q, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load apps")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching apps. Error: "+err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Apps fetched successfully",
		pagedResponse{Content: &apps, Pagination: &paginationData}, http.StatusOK))
}

// CreateApp
// @Summary Create an application
// @Description This endpoint creates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param application body models.Application true "Application Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Application}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications [post]
func (a *ApplicationHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var newApp models.Application
	err := util.ReadJSON(r, &newApp)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := m.GetGroupFromContext(r.Context())
	appService := createApplicationService(a)
	app, err := appService.CreateApp(r.Context(), &newApp, group)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App created successfully", app, http.StatusCreated))
}

// UpdateApp
// @Summary Update an application
// @Description This endpoint updates an application
// @Tags Application
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Param application body models.Application true "Application Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Application}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID} [put]
func (a *ApplicationHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var appUpdate models.UpdateApplication
	err := util.ReadJSON(r, &appUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := m.GetApplicationFromContext(r.Context())
	appService := createApplicationService(a)

	err = appService.UpdateApplication(r.Context(), &appUpdate, app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App updated successfully", app, http.StatusAccepted))
}

// DeleteApp
// @Summary Delete app
// @Description This endpoint deletes an app
// @Tags Application
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID} [delete]
func (a *ApplicationHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	app := m.GetApplicationFromContext(r.Context())
	appService := createApplicationService(a)

	err := appService.DeleteApplication(r.Context(), app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App deleted successfully", nil, http.StatusOK))
}

// CreateAppEndpoint
// @Summary Create an application endpoint
// @Description This endpoint creates an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints [post]
func (a *ApplicationHandler) CreateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := m.ParseEndpointFromBody(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := m.GetApplicationFromContext(r.Context())
	appService := createApplicationService(a)

	endpoint, err := appService.CreateAppEndpoint(r.Context(), e, app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App endpoint created successfully", endpoint, http.StatusCreated))
}

// GetAppEndpoint
// @Summary Get application endpoint
// @Description This endpoint fetches an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints/{endpointID} [get]
func (a *ApplicationHandler) GetAppEndpoint(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("App endpoint fetched successfully",
		*m.GetApplicationFromContext(r.Context()), http.StatusOK))
}

// GetAppEndpoints
// @Summary Get application endpoints
// @Description This endpoint fetches an application's endpoints
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints [get]
func (a *ApplicationHandler) GetAppEndpoints(w http.ResponseWriter, r *http.Request) {
	app := m.GetApplicationFromContext(r.Context())

	app.Endpoints = m.FilterDeletedEndpoints(app.Endpoints)
	_ = render.Render(w, r, util.NewServerResponse("App endpoints fetched successfully", app.Endpoints, http.StatusOK))
}

// UpdateAppEndpoint
// @Summary Update an application endpoint
// @Description This endpoint updates an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints/{endpointID} [put]
func (a *ApplicationHandler) UpdateAppEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := m.ParseEndpointFromBody(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := m.GetApplicationFromContext(r.Context())
	endPointId := chi.URLParam(r, "endpointID")
	appService := createApplicationService(a)

	endpoint, err := appService.UpdateAppEndpoint(r.Context(), e, endPointId, app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Apps endpoint updated successfully", endpoint, http.StatusAccepted))
}

// DeleteAppEndpoint
// @Summary Delete application endpoint
// @Description This endpoint deletes an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints/{endpointID} [delete]
func (a *ApplicationHandler) DeleteAppEndpoint(w http.ResponseWriter, r *http.Request) {
	app := m.GetApplicationFromContext(r.Context())
	e := m.GetApplicationEndpointFromContext(r.Context())
	appService := createApplicationService(a)

	err := appService.DeleteAppEndpoint(r.Context(), e, app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App endpoint deleted successfully", nil, http.StatusOK))
}

// CreateEndpointSecret
// @Summary Create application endpoint secret
// @Description This endpoint creates an endpoint secret
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints/{endpointID}/secrets [post]
func (a *ApplicationHandler) CreateEndpointSecret(w http.ResponseWriter, r *http.Request) {
	s := &models.EndpointSecret{}
	err := util.ReadJSON(r, s)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app := m.GetApplicationFromContext(r.Context())

	as := createApplicationService(a)
	_, err = as.CreateAppEndpointSecret(r.Context(), s, chi.URLParam(r, "endpointID"), app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint secret created successfully",
		*m.GetApplicationFromContext(r.Context()), http.StatusCreated))
}

// ExpireEndpointSecret
// @Summary Get application endpoint secret
// @Description This endpoint fetches an application endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param appID path string true "application id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/applications/{appID}/endpoints/{endpointID}/secrets/{secretID}/expire [delete]
func (a *ApplicationHandler) ExpireEndpointSecret(w http.ResponseWriter, r *http.Request) {
	app := m.GetApplicationFromContext(r.Context())

	as := createApplicationService(a)
	err := as.ExpireEndpointSecret(r.Context(), chi.URLParam(r, "secretID"), chi.URLParam(r, "endpointID"), app)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		nil, http.StatusOK))
}

func (a *ApplicationHandler) GetPaginatedApps(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Apps fetched successfully",
		pagedResponse{
			Content:    *m.GetApplicationsFromContext(r.Context()),
			Pagination: m.GetPaginationDataFromContext(r.Context()),
		}, http.StatusOK))
}
