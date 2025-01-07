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

func (h *Handler) CreateInstanceOverrides(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceOverrides
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaultsRepo := postgres.NewInstanceOverridesRepo(h.A.DB)

	var result *datastore.InstanceOverrides
	if result, err = defaultsRepo.Create(r.Context(), &instanceDefaults); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnprocessableEntity))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default created successfully", result, http.StatusOK))
}

func (h *Handler) UpdateInstanceOverrides(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceOverrides
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	defaults, err := h.retrieveInstanceOverrides(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaultsRepo := postgres.NewInstanceOverridesRepo(h.A.DB)

	var result *datastore.InstanceOverrides
	if result, err = defaultsRepo.Update(r.Context(), defaults.UID, &instanceDefaults); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnprocessableEntity))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default updated successfully", result, http.StatusOK))
}

func (h *Handler) GetInstanceOverrides(w http.ResponseWriter, r *http.Request) {
	var instanceDefaults datastore.InstanceOverrides
	err := util.ReadJSON(r, &instanceDefaults)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	defaults, err := h.retrieveInstanceOverrides(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Instance default fetched successfully", defaults, http.StatusOK))
}

func (h *Handler) retrieveInstanceOverrides(r *http.Request) (*datastore.InstanceOverrides, error) {
	id := chi.URLParam(r, "configID")

	if util.IsStringEmpty(id) {
		id = r.URL.Query().Get("configID")
	}

	defaultsRepo := postgres.NewInstanceOverridesRepo(h.A.DB)
	return defaultsRepo.FetchByID(r.Context(), id)
}

func (h *Handler) GetInstanceOverridesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	if pageable.PrevCursor == "" {
		pageable.NextCursor = "0"
	}

	overridesRepo := postgres.NewInstanceOverridesRepo(h.A.DB)

	overrides, paginationData, err := overridesRepo.LoadPaged(r.Context(), pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch instance overrides")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Data fetched successfully",
		models.PagedResponse{Content: &overrides, Pagination: &paginationData}, http.StatusOK))
}
