package public

import (
	"github.com/frain-dev/convoy/api/models"
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
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
// @Param request query models.QueryListMetaEvent false "Query Params"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.MetaEventResponse}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events [get]
func (a *PublicHandler) GetMetaEventsPaged(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListMetaEvent
	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	metaEvents, paginationData, err := postgres.NewMetaEventRepo(a.A.DB, a.A.Cache).LoadMetaEventsPaged(r.Context(), project.UID, data.Filter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching meta events", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(metaEvents, func(metaEvent datastore.MetaEvent) models.MetaEventResponse {
		return models.MetaEventResponse{MetaEvent: &metaEvent}
	})
	_ = render.Render(w, r, util.NewServerResponse("Meta events fetched successfully",
		pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

// GetMetaEvent
// @Summary Retrieve a meta event
// @Description This endpoint retrieves a meta event
// @Tags Meta Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param metaEventID path string true "meta event id"
// @Success 200 {object} util.ServerResponse{data=models.MetaEventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events/{metaEventID} [get]
func (a *PublicHandler) GetMetaEvent(w http.ResponseWriter, r *http.Request) {
	metaEvent, err := a.retrieveMetaEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.MetaEventResponse{MetaEvent: metaEvent}
	_ = render.Render(w, r, util.NewServerResponse("Meta event fetched successfully",
		resp, http.StatusOK))
}

// ResendMetaEvent
// @Summary Retry meta event
// @Description This endpoint retries a meta event
// @Tags Meta Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param metaEventID path string true "meta event id"
// @Success 200 {object} util.ServerResponse{data=models.MetaEventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/meta-events/{metaEventID}/resend [put]
func (a *PublicHandler) ResendMetaEvent(w http.ResponseWriter, r *http.Request) {
	metaEvent, err := a.retrieveMetaEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	metaEventRepo := postgres.NewMetaEventRepo(a.A.DB, a.A.Cache)
	metaEventService := &services.MetaEventService{Queue: a.A.Queue, MetaEventRepo: metaEventRepo}
	err = metaEventService.Run(r.Context(), metaEvent)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.MetaEventResponse{MetaEvent: metaEvent}
	_ = render.Render(w, r, util.NewServerResponse("Meta event processed for retry successfully", resp, http.StatusOK))
}

func (a *PublicHandler) retrieveMetaEvent(r *http.Request) (*datastore.MetaEvent, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.MetaEvent{}, err
	}

	metaEventID := chi.URLParam(r, "metaEventID")
	metaEventRepo := postgres.NewMetaEventRepo(a.A.DB, a.A.Cache)
	return metaEventRepo.FindMetaEventByID(r.Context(), project.UID, metaEventID)
}
