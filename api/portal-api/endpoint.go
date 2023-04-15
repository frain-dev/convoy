package portalapi

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

func createEndpointService(a *PortalLinkHandler) *services.EndpointService {
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

func (a *PortalLinkHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
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

func (a *PortalLinkHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", endpoint, http.StatusOK))
}

func (a *PortalLinkHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
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

func (a *PortalLinkHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var e models.UpdateEndpoint

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

	endpoint, err := a.retrieveEndpoint(r)
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

func (a *PortalLinkHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	endpointService := createEndpointService(a)
	err = endpointService.DeleteEndpoint(r.Context(), endpoint, project)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete endpoint")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

func (a *PortalLinkHandler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
	var e *models.ExpireSecret
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

	endpoint, err := a.retrieveEndpoint(r)
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

func (a *PortalLinkHandler) ToggleEndpointStatus(w http.ResponseWriter, r *http.Request) {
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

func (a *PortalLinkHandler) retrieveEndpoint(r *http.Request) (*datastore.Endpoint, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	endpointID := chi.URLParam(r, "endpointID")
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	return endpointRepo.FindEndpointByID(r.Context(), endpointID, project.UID)
}
