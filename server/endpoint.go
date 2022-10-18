package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"

	"github.com/go-chi/render"
)

func createEndpointService(a *ApplicationHandler) *services.EndpointService {
	endpointRepo := mongo.NewEndpointRepo(a.A.Store)
	eventRepo := mongo.NewEventRepository(a.A.Store)
	eventDeliveryRepo := mongo.NewEventDeliveryRepository(a.A.Store)

	return services.NewEndpointService(
		endpointRepo, eventRepo, eventDeliveryRepo, a.A.Cache,
	)
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

// CreateEndpoint
// @Summary Create an endpoint
// @Description This endpoint creates an endpoint
// @Tags Application Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/endpoints [post]
func (a *ApplicationHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := m.ParseEndpointFromBody(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := m.GetGroupFromContext(r.Context())
	endpointService := createEndpointService(a)

	endpoint, err := endpointService.CreateEndpoint(r.Context(), e, group.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint created successfully", endpoint, http.StatusCreated))
}

// GetEndpoint
// @Summary Get endpoint
// @Description This endpoint fetches an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/endpoints/{endpointID} [get]
func (a *ApplicationHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully",
		*m.GetEndpointFromContext(r.Context()), http.StatusOK))
}

// GetAppEndpoints
// @Summary Get endpoints
// @Description This endpoint fetches an endpoints
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/endpoints [get]
func (a *ApplicationHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	endpointRepo := mongo.NewEndpointRepo(a.A.Store)
	q := r.URL.Query().Get("q")
	pageable := m.GetPageableFromContext(r.Context())

	endpoints, paginationData, err := endpointRepo.LoadEndpointsPaged(r.Context(), group.UID, q, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully",
		pagedResponse{Content: &endpoints, Pagination: &paginationData}, http.StatusOK))
}

// UpdateEndpoint
// @Summary Update an endpoint
// @Description This endpoint updates an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param endpointID path string true "endpoint id"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/endpoints/{endpointID} [put]
func (a *ApplicationHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	e, err := m.ParseEndpointFromBody(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint := m.GetEndpointFromContext(r.Context())
	endpointService := createEndpointService(a)

	endpoint, err = endpointService.UpdateEndpoint(r.Context(), e, endpoint)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints endpoint updated successfully", endpoint, http.StatusAccepted))
}

// DeleteEndpoint
// @Summary Delete endpoint
// @Description This endpoint deletes an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param endpointID path string true "endpoint id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/endpoints/{endpointID} [delete]
func (a *ApplicationHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint := m.GetEndpointFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())
	endpointService := createEndpointService(a)

	err := endpointService.DeleteEndpoint(r.Context(), endpoint, group.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

func (a *ApplicationHandler) GetPaginatedEndpoints(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully",
		pagedResponse{Content: *m.GetEndpointsFromContext(r.Context()),
			Pagination: m.GetPaginationDataFromContext(r.Context())}, http.StatusOK))
}
