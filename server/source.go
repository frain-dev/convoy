package server

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateSource
// @Summary Create a source
// @Description This endpoint creates a source
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param source body models.Source true "Source Details"
// @Success 200 {object} serverResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources [post]
func (a *applicationHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var newSource models.Source
	if err := util.ReadJSON(r, &newSource); err != nil {
		fmt.Println("err is", err)
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := getGroupFromContext(r.Context())

	source, err := a.sourceService.CreateSource(r.Context(), &newSource, group)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	baseUrl := getBaseUrlFromContext(r.Context())
	s := sourceResponse(source, baseUrl)
	_ = render.Render(w, r, newServerResponse("Source created successfully", s, http.StatusCreated))
}

// GetSource
// @Summary Get a source
// @Description This endpoint fetches a source by its id
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param sourceID path string true "source id"
// @Success 200 {object} serverResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [get]
func (a *applicationHandler) GetSourceByID(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())

	source, err := a.sourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	baseUrl := getBaseUrlFromContext(r.Context())
	s := sourceResponse(source, baseUrl)

	_ = render.Render(w, r, newServerResponse("Source fetched successfully", s, http.StatusOK))
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
// @Success 200 {object} serverResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [put]
func (a *applicationHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var sourceUpdate models.UpdateSource
	err := util.ReadJSON(r, &sourceUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	group := getGroupFromContext(r.Context())
	source, err := a.sourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	source, err = a.sourceService.UpdateSource(r.Context(), group, &sourceUpdate, source)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	baseUrl := getBaseUrlFromContext(r.Context())
	s := sourceResponse(source, baseUrl)

	_ = render.Render(w, r, newServerResponse("Source updated successfully", s, http.StatusAccepted))
}

// DeleteSource
// @Summary Delete source
// @Description This endpoint deletes a source
// @Tags Source
// @Accept  json
// @Produce  json
// @Param groupId query string true "group id"
// @Param sourceID path string true "source id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources/{sourceID} [delete]
func (a *applicationHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())
	source, err := a.sourceService.FindSourceByID(r.Context(), group, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	err = a.sourceService.DeleteSourceByID(r.Context(), group, source.UID)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Source deleted successfully", nil, http.StatusOK))
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
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]models.SourceResponse}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /sources [get]
func (a *applicationHandler) LoadSourcesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())

	f := &datastore.SourceFilter{
		Type: r.URL.Query().Get("type"),
	}

	sources, paginationData, err := a.sourceService.LoadSourcesPaged(r.Context(), group, f, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching sources", http.StatusInternalServerError))
		return
	}

	sourcesResponse := []*models.SourceResponse{}
	baseUrl := getBaseUrlFromContext(r.Context())

	for _, source := range sources {
		s := sourceResponse(&source, baseUrl)
		sourcesResponse = append(sourcesResponse, s)
	}

	_ = render.Render(w, r, newServerResponse("Sources fetched successfully", pagedResponse{Content: sourcesResponse, Pagination: &paginationData}, http.StatusOK))
}

func sourceResponse(s *datastore.Source, baseUrl string) *models.SourceResponse {
	return &models.SourceResponse{
		UID:        s.UID,
		MaskID:     s.MaskID,
		GroupID:    s.GroupID,
		Name:       s.Name,
		Type:       s.Type,
		URL:        fmt.Sprintf("%s/ingester/%s", baseUrl, s.MaskID),
		IsDisabled: s.IsDisabled,
		Verifier:   s.Verifier,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
		DeletedAt:  s.DeletedAt,
	}
}
