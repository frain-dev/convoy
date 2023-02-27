package server

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSourceService(a *ApplicationHandler) *services.SourceService {
	sourceRepo := mongo.NewSourceRepo(a.A.Store)

	return services.NewSourceService(sourceRepo, a.A.Cache)
}

// CreateSource
// @Summary Create a source
// @Description This endpoint creates a source
// @Tags Sources
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param source body models.Source true "Source Details"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/sources [post]
func (a *ApplicationHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var newSource models.Source
	if err := util.ReadJSON(r, &newSource); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())

	sourceService := createSourceService(a)
	source, err := sourceService.CreateSource(r.Context(), &newSource, project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgService := createOrganisationService(a)
	org, err := orgService.FindOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	fillSourceURL(source, baseUrl, org.CustomDomain)
	_ = render.Render(w, r, util.NewServerResponse("Source created successfully", source, http.StatusCreated))
}

// GetSourceByID
// @Summary Get a source
// @Description This endpoint fetches a source by its id
// @Tags Sources
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param sourceID path string true "source id"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/sources/{sourceID} [get]
func (a *ApplicationHandler) GetSourceByID(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())

	sourceService := createSourceService(a)
	source, err := sourceService.FindSourceByID(r.Context(), project, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgService := createOrganisationService(a)
	org, err := orgService.FindOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	fillSourceURL(source, baseUrl, org.CustomDomain)

	_ = render.Render(w, r, util.NewServerResponse("Source fetched successfully", source, http.StatusOK))
}

// UpdateSource
// @Summary Update a source
// @Description This endpoint updates a source
// @Tags Sources
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param sourceID path string true "source id"
// @Param source body models.Source true "Source Details"
// @Success 200 {object} util.ServerResponse{data=models.SourceResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/sources/{sourceID} [put]
func (a *ApplicationHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var sourceUpdate models.UpdateSource
	err := util.ReadJSON(r, &sourceUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project := m.GetProjectFromContext(r.Context())
	sourceService := createSourceService(a)

	source, err := sourceService.FindSourceByID(r.Context(), project, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	source, err = sourceService.UpdateSource(r.Context(), project, &sourceUpdate, source)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgService := createOrganisationService(a)
	org, err := orgService.FindOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	fillSourceURL(source, baseUrl, org.CustomDomain)

	_ = render.Render(w, r, util.NewServerResponse("Source updated successfully", source, http.StatusAccepted))
}

// DeleteSource
// @Summary Delete source
// @Description This endpoint deletes a source
// @Tags Sources
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param sourceID path string true "source id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/sources/{sourceID} [delete]
func (a *ApplicationHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	sourceService := createSourceService(a)

	source, err := sourceService.FindSourceByID(r.Context(), project, chi.URLParam(r, "sourceID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = sourceService.DeleteSource(r.Context(), project, source)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Source deleted successfully", nil, http.StatusOK))
}

// LoadSourcesPaged
// @Summary Fetch multiple sources
// @Description This endpoint fetches multiple sources
// @Tags Sources
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.SourceResponse}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/sources [get]
func (a *ApplicationHandler) LoadSourcesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())

	f := &datastore.SourceFilter{
		Type: r.URL.Query().Get("type"),
	}

	sourceService := createSourceService(a)
	sources, paginationData, err := sourceService.LoadSourcesPaged(r.Context(), project, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching sources", http.StatusInternalServerError))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())

	orgService := createOrganisationService(a)
	org, err := orgService.FindOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	for i := range sources {
		fillSourceURL(&sources[i], baseUrl, org.CustomDomain)
	}

	_ = render.Render(w, r, util.NewServerResponse("Sources fetched successfully", pagedResponse{Content: sources, Pagination: &paginationData}, http.StatusOK))
}

func fillSourceURL(s *datastore.Source, baseUrl string, customDomain string) {
	url := baseUrl
	if len(customDomain) > 0 {
		url = customDomain
	}

	s.URL = fmt.Sprintf("%s/ingest/%s", url, s.MaskID)
}
