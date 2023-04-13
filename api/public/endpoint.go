package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"

	"github.com/go-chi/render"
)

func createEndpointService(a *PublicHandler) *services.EndpointService {
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	eventRepo := postgres.NewEventRepo(a.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)
	projectRepo := postgres.NewProjectRepo(a.A.DB)

	return services.NewEndpointService(
		projectRepo, endpointRepo, eventRepo, eventDeliveryRepo, a.A.Cache, a.A.Queue,
	)
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

// CreateEndpoint
// @Summary Create an endpoint
// @Description This endpoint creates an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints [post]
func (a *PublicHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.Endpoint
	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err := endpointService.CreateEndpoint(r.Context(), e, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint created successfully", endpoint, http.StatusCreated))
}

// GetEndpoint
// @Summary Retrieve endpoint
// @Description This endpoint fetches an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID} [get]
func (a *PublicHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", endpoint, http.StatusOK))
}

// GetEndpoints
// @Summary List all endpoints
// @Description This endpoint fetches an endpoints
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints [get]
func (a *PublicHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	filter := &datastore.Filter{
		Query:   r.URL.Query().Get("q"),
		OwnerID: r.URL.Query().Get("ownerId"),
	}

	pageable := m.GetPageableFromContext(r.Context())
	endpoints, paginationData, err := endpointService.LoadEndpointsPaged(r.Context(), project.UID, filter, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load endpoints")
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
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Param endpoint body models.Endpoint true "Endpoint Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID} [put]
func (a *PublicHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.UpdateEndpoint

	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err = endpointService.UpdateEndpoint(r.Context(), e, endpoint, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint updated successfully", endpoint, http.StatusAccepted))
}

// DeleteEndpoint
// @Summary Delete endpoint
// @Description This endpoint deletes an endpoint
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID} [delete]
func (a *PublicHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	err := endpointService.DeleteEndpoint(r.Context(), endpoint, project)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete endpoint")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

// ExpireSecret
// @Summary Roll endpoint secret
// @Description This endpoint expires and re-generates the endpoint secret.
// @Tags Endpoints
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Param endpoint body models.ExpireSecret true "Expire Secret Body Parameters"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID}/expire_secret [put]
func (a *PublicHandler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
	var e *models.ExpireSecret
	err := util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	endpoint, err = endpointService.ExpireSecret(r.Context(), e, endpoint, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		endpoint, http.StatusOK))
}

// ToggleEndpointStatus
// @Summary Toggle endpoint status
// @Description This endpoint toggles an endpoint status between the active and inactive statetes
// @Tags Endpoints
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID}/toggle_status [put]
func (a *PublicHandler) ToggleEndpointStatus(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointService := createEndpointService(a)
	endpoint, err := endpointService.ToggleEndpointStatus(r.Context(), project.UID, endpointID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", endpoint, http.StatusAccepted))
}

// PauseEndpoint
// @Summary Pause endpoint
// @Description This endpoint toggles an endpoint status between the active and paused states
// @Tags Endpoints
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param endpointID path string true "Endpoint ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Endpoint}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/endpoints/{endpointID}/pause [put]
func (a *PublicHandler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointService := createEndpointService(a)
	endpoint, err := endpointService.PauseEndpoint(r.Context(), p.UID, endpointID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", endpoint, http.StatusAccepted))
}

func (a *DashboardHandler) retrieveEndpoint(r *http.Request) (*datastore.Endpoint, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	return endpointRepo.FindEndpointByID(r.Context(), endpointID, project.UID)
}
