package server

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

// CreateSource
// @Summary Create a source
// @Description This endpoint creates a source
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param source body models.Source true "Source Details"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources [post]
func (a *ApplicationHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var newSource models.Source
	if err := util.ReadJSON(r, &newSource); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := m.GetGroupFromContext(r.Context())

	source, err := a.S.SourceService.CreateSource(r.Context(), &newSource, group)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	sr := sourceResponse(source, baseUrl)
	_ = render.Render(w, r, util.NewServerResponse("Source created successfully", sr, http.StatusCreated))
}

// GetSource
// @Summary Get a source
// @Description This endpoint fetches a source by its id
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param sourceID path string true "source id"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [get]
func (a *ApplicationHandler) GetSourceByID(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())

	source, err := a.S.SourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	sr := sourceResponse(source, baseUrl)

	_ = render.Render(w, r, util.NewServerResponse("Source fetched successfully", sr, http.StatusOK))
}

// UpdateSource
// @Summary Update a source
// @Description This endpoint updates a source
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param sourceID path string true "source id"
// @Param source body models.Source true "Source Details"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [put]
func (a *ApplicationHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var sourceUpdate models.UpdateSource
	err := util.ReadJSON(r, &sourceUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := m.GetGroupFromContext(r.Context())
	source, err := a.S.SourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	source, err = a.S.SourceService.UpdateSource(r.Context(), group, &sourceUpdate, source)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	sr := sourceResponse(source, baseUrl)

	_ = render.Render(w, r, util.NewServerResponse("Source updated successfully", sr, http.StatusAccepted))
}

// DeleteSource
// @Summary Delete source
// @Description This endpoint deletes a source
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param sourceID path string true "source id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [delete]
func (a *ApplicationHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	source, err := a.S.SourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = a.S.SourceService.DeleteSource(r.Context(), group, source)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Source deleted successfully", nil, http.StatusOK))
}

// LoadSourcesPaged
// @Summary Fetch multiple sources
// @Description This endpoint fetches multiple sources
// @Tags Source
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.SourceResponse}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources [get]
func (a *ApplicationHandler) LoadSourcesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())

	f := &datastore.SourceFilter{
		Type: r.URL.Query().Get("type"),
	}

	sources, paginationData, err := a.S.SourceService.LoadSourcesPaged(r.Context(), group, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching sources", http.StatusInternalServerError))
		return
	}

	sourcesResponse := []*models.SourceResponse{}
	baseUrl := m.GetHostFromContext(r.Context())

	for _, source := range sources {
		s := sourceResponse(&source, baseUrl)
		sourcesResponse = append(sourcesResponse, s)
	}

	_ = render.Render(w, r, util.NewServerResponse("Sources fetched successfully", pagedResponse{Content: sourcesResponse, Pagination: &paginationData}, http.StatusOK))
}

func sourceResponse(s *datastore.Source, baseUrl string) *models.SourceResponse {
	return &models.SourceResponse{
		UID:            s.UID,
		MaskID:         s.MaskID,
		GroupID:        s.GroupID,
		Name:           s.Name,
		Type:           s.Type,
		Provider:       s.Provider,
		ProviderConfig: s.ProviderConfig,
		URL:            fmt.Sprintf("%s/ingest/%s", baseUrl, s.MaskID),
		IsDisabled:     s.IsDisabled,
		Verifier:       s.Verifier,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		DeletedAt:      s.DeletedAt,
	}
}
