package public

import (
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// GetMetaEventsPaged
// @Summary List all meta events
// @Description This endpoint fetches meta events with pagination
// @Tags Meta Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.MetaEvent{data=Stub}}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events [get]
func (a *PublicHandler) GetMetaEventsPaged(w http.ResponseWriter, r *http.Request) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	f := &datastore.Filter{SearchParams: searchParams, Pageable: pageable}
	metaEvents, paginationData, err := postgres.NewMetaEventRepo(a.A.DB).LoadMetaEventsPaged(r.Context(), project.UID, f)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching meta events", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Meta events fetched successfully",
		pagedResponse{Content: metaEvents, Pagination: &paginationData}, http.StatusOK))
}

// GetMetaEvent
// @Summary Retrieve a meta event
// @Description This endpoint retrieves a meta event
// @Tags Meta Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param metaEventID path string true "meta event id"
// @Success 200 {object} util.ServerResponse{data=datastore.MetaEvent{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events/{metaEventID} [get]
func (a *PublicHandler) GetMetaEvent(w http.ResponseWriter, r *http.Request) {
	metEvent, err := a.retrieveMetaEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Meta event fetched successfully",
		metEvent, http.StatusOK))
}

// ResendMetaEvent
// @Summary Retry meta event
// @Description This endpoint retries a meta event
// @Tags Meta Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param metaEventID path string true "meta event id"
// @Success 200 {object} util.ServerResponse{data=datastore.MetaEvent{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events/{metaEventID}/resend [put]
func (a *PublicHandler) ResendMetaEvent(w http.ResponseWriter, r *http.Request) {
	metaEvent, err := a.retrieveMetaEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	metaEventRepo := postgres.NewMetaEventRepo(a.A.DB)
	metaEventService := &services.MetaEventService{Queue: a.A.Queue, MetaEventRepo: metaEventRepo}
	err = metaEventService.Run(r.Context(), metaEvent)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Meta event processed for retry successfully", metaEvent, http.StatusOK))
}

func (a *PublicHandler) retrieveMetaEvent(r *http.Request) (*datastore.MetaEvent, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.MetaEvent{}, err
	}

	metaEventID := chi.URLParam(r, "metaEventID")
	metaEventRepo := postgres.NewMetaEventRepo(a.A.DB)
	return metaEventRepo.FindMetaEventByID(r.Context(), project.UID, metaEventID)
}
