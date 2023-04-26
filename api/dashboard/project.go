package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createProjectService(a *DashboardHandler) *services.ProjectService {
	apiKeyRepo := postgres.NewAPIKeyRepo(a.A.DB)
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	eventRepo := postgres.NewEventRepo(a.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)

	return services.NewProjectService(
		apiKeyRepo, projectRepo, eventRepo,
		eventDeliveryRepo, a.A.Limiter, a.A.Cache,
	)
}

func (a *DashboardHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", project, http.StatusOK))
}

func (a *DashboardHandler) GetProjectStatistics(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
		return
	}

	projectService := createProjectService(a)

	err = projectService.FillProjectStatistics(r.Context(), project)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project Stats fetched successfully", project.Statistics, http.StatusOK))
}

func (a *DashboardHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
		return
	}

	projectService := createProjectService(a)

	err = projectService.DeleteProject(r.Context(), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project deleted successfully",
		nil, http.StatusOK))
}

func (a *DashboardHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var newProject models.Project
	err := util.ReadJSON(r, &newProject)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
		return
	}

	member, err := a.retrieveMembership(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	projectService := createProjectService(a)
	project, apiKey, err := projectService.CreateProject(r.Context(), &newProject, org, member)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.CreateProjectResponse{
		APIKey:  apiKey,
		Project: project,
	}

	_ = render.Render(w, r, util.NewServerResponse("Project created successfully", resp, http.StatusCreated))
}

func (a *DashboardHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateProject
	err := util.ReadJSON(r, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	p, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", p); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
		return
	}

	projectService := createProjectService(a)
	project, err := projectService.UpdateProject(r.Context(), p, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project updated successfully", project, http.StatusAccepted))
}

func (a *DashboardHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projectService := createProjectService(a)

	projects, err := projectService.GetProjects(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Projects fetched successfully", projects, http.StatusOK))
}
