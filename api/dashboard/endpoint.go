package dashboard

import (
	"net/http"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"

	"github.com/go-chi/render"
)

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

func (a *DashboardHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
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

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	err = util.Validate(e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ce := services.CreateEndpointService{
		Cache:          a.A.Cache,
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:    postgres.NewProjectRepo(a.A.DB),
		PortalLinkRepo: postgres.NewPortalLinkRepo(a.A.DB),
		E:              e,
		ProjectID:      project.UID,
	}

	endpoint, err := ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint created successfully", endpoint, http.StatusCreated))
}

func (a *DashboardHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	endpoint, err := a.retrieveEndpoint(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint fetched successfully", endpoint, http.StatusOK))
}

func (a *DashboardHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	filter := &datastore.Filter{
		Query:   r.URL.Query().Get("q"),
		OwnerID: r.URL.Query().Get("ownerId"),
	}

	pageable := m.GetPageableFromContext(r.Context())
	filter.Query = strings.TrimSpace(filter.Query)
	endpoints, paginationData, err := postgres.NewEndpointRepo(a.A.DB).LoadEndpointsPaged(r.Context(), project.UID, filter, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load endpoints")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoints fetched successfully",
		pagedResponse{Content: &endpoints, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
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

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	err = util.Validate(e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ce := services.UpdateEndpointService{
		Cache:        a.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB),
		E:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint updated successfully", endpoint, http.StatusAccepted))
}

func (a *DashboardHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
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

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	err = postgres.NewEndpointRepo(a.A.DB).DeleteEndpoint(r.Context(), endpoint, project.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete endpoint")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete endpoint", http.StatusBadRequest))
		return
	}

	endpointCacheKey := convoy.EndpointsCacheKey.Get(endpoint.UID).String()
	err = a.A.Cache.Delete(r.Context(), endpointCacheKey)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete endpoint cache")
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
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

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	xs := services.ExpireSecretService{
		Queuer:       a.A.Queue,
		Cache:        a.A.Cache,
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectRepo:  postgres.NewProjectRepo(a.A.DB),
		S:            e,
		Endpoint:     endpoint,
		Project:      project,
	}

	endpoint, err = xs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		endpoint, http.StatusOK))
}

func (a *DashboardHandler) ToggleEndpointStatus(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	te := services.ToggleEndpointStatusService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}

	endpoint, err := te.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("endpoint status updated successfully", endpoint, http.StatusAccepted))
}

func (a *DashboardHandler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	ps := services.PauseEndpointService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		ProjectID:    project.UID,
		EndpointId:   chi.URLParam(r, "endpointID"),
	}

	endpoint, err := ps.Run(r.Context())
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

	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	endpointID := chi.URLParam(r, "endpointID")
	return endpointRepo.FindEndpointByID(r.Context(), endpointID, project.UID)
}
