package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (a *DashboardHandler) GetMetaEventsPaged(w http.ResponseWriter, r *http.Request) {
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

func (a *DashboardHandler) GetMetaEvent(w http.ResponseWriter, r *http.Request) {
	metEvent, err := a.retrieveMetaEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Meta event fetched successfully",
		metEvent, http.StatusOK))
}

func (a *DashboardHandler) retrieveMetaEvent(r *http.Request) (*datastore.MetaEvent, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.MetaEvent{}, err
	}

	metaEventID := chi.URLParam(r, "metaEventID")
	metaEventRepo := postgres.NewMetaEventRepo(a.A.DB)
	return metaEventRepo.FindMetaEventByID(r.Context(), project.UID, metaEventID)
}
