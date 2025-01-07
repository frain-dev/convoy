package handlers

import (
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (h *Handler) CreateInstanceDefaults(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceDefaults
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaultsRepo := postgres.NewInstanceDefaultsRepo(h.A.DB)

	var result *datastore.InstanceDefaults
	if result, err = defaultsRepo.Create(r.Context(), &instanceDefaults); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnprocessableEntity))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default created successfully", result, http.StatusOK))
}

func (h *Handler) UpdateInstanceDefaults(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceDefaults
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	defaults, err := h.retrieveInstanceDefaults(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaultsRepo := postgres.NewInstanceDefaultsRepo(h.A.DB)

	var result *datastore.InstanceDefaults
	if result, err = defaultsRepo.Update(r.Context(), defaults.UID, &instanceDefaults); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnprocessableEntity))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default updated successfully", result, http.StatusOK))
}

func (h *Handler) GetInstanceDefaults(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceDefaults
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaults, err := h.retrieveInstanceDefaults(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default fetched successfully", defaults, http.StatusOK))
}

func (h *Handler) retrieveInstanceDefaults(r *http.Request) (*datastore.InstanceDefaults, error) {
	id := chi.URLParam(r, "configID")

	if util.IsStringEmpty(id) {
		id = r.URL.Query().Get("configID")
	}

	defaultsRepo := postgres.NewInstanceDefaultsRepo(h.A.DB)
	return defaultsRepo.FetchByID(r.Context(), id)
}

func (h *Handler) GetInstanceDefaultsPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	if pageable.PrevCursor == "" {
		pageable.NextCursor = "0"
	}

	defaultsRepo := postgres.NewInstanceDefaultsRepo(h.A.DB)

	defaults, paginationData, err := defaultsRepo.LoadPaged(r.Context(), pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch instance defaults")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Data fetched successfully",
		models.PagedResponse{Content: &defaults, Pagination: &paginationData}, http.StatusOK))
}
