package dashboard

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSourceService(a *DashboardHandler) *services.SourceService {
	sourceRepo := postgres.NewSourceRepo(a.A.DB)

	return services.NewSourceService(sourceRepo, a.A.Cache)
}

func (a *DashboardHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var newSource models.Source
	if err := util.ReadJSON(r, &newSource); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

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

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())
	_ = render.Render(w, r, util.NewServerResponse("Source created successfully", source, http.StatusCreated))
}

func (a *DashboardHandler) GetSourceByID(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

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

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())
	_ = render.Render(w, r, util.NewServerResponse("Source fetched successfully", source, http.StatusOK))
}

func (a *DashboardHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var sourceUpdate models.UpdateSource
	err := util.ReadJSON(r, &sourceUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

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

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())

	_ = render.Render(w, r, util.NewServerResponse("Source updated successfully", source, http.StatusAccepted))
}

func (a *DashboardHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
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

func (a *DashboardHandler) LoadSourcesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	f := &datastore.SourceFilter{
		Type: r.URL.Query().Get("type"),
	}

	sourceService := createSourceService(a)
	sources, paginationData, err := sourceService.LoadSourcesPaged(r.Context(), project, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var customDomain string
	if org == nil {
		customDomain = ""
	} else {
		customDomain = org.CustomDomain.ValueOrZero()
	}

	for i := range sources {
		fillSourceURL(&sources[i], baseUrl, customDomain)
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
