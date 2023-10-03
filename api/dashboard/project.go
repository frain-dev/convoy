package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createProjectService(a *DashboardHandler) (*services.ProjectService, error) {
	apiKeyRepo := postgres.NewAPIKeyRepo(a.A.DB, a.A.Cache)
	projectRepo := postgres.NewProjectRepo(a.A.DB, a.A.Cache)
	eventRepo := postgres.NewEventRepo(a.A.DB, a.A.Cache)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB, a.A.Cache)

	projectService, err := services.NewProjectService(
		apiKeyRepo, projectRepo, eventRepo,
		eventDeliveryRepo, a.A.Cache,
	)

	if err != nil {
		return nil, err
	}

	return projectService, nil
}

func (a *DashboardHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.ProjectResponse{Project: project}
	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", resp, http.StatusOK))
}

func (a *DashboardHandler) GetProjectStatistics(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = postgres.NewProjectRepo(a.A.DB, a.A.Cache).FillProjectsStatistics(r.Context(), project)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to count project statistics")
		_ = render.Render(w, r, util.NewErrorResponse("failed to count project statistics", http.StatusBadRequest))
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
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	err = postgres.NewProjectRepo(a.A.DB, a.A.Cache).DeleteProject(r.Context(), project.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete project")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete project", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project deleted successfully",
		nil, http.StatusOK))
}

func (a *DashboardHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var newProject models.CreateProject
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
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	member, err := a.retrieveMembership(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err := newProject.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	projectService, err := createProjectService(a)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, apiKey, err := projectService.CreateProject(r.Context(), &newProject, org, member)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.CreateProjectResponse{
		APIKey:  apiKey,
		Project: &models.ProjectResponse{Project: project},
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
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	if err := update.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	projectService, err := createProjectService(a)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := projectService.UpdateProject(r.Context(), p, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.ProjectResponse{Project: project}
	_ = render.Render(w, r, util.NewServerResponse("Project updated successfully", resp, http.StatusAccepted))
}

func (a *DashboardHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projects, err := postgres.NewProjectRepo(a.A.DB, a.A.Cache).LoadProjects(r.Context(), filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load projects")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching projects", http.StatusBadRequest))
		return
	}

	resp := models.NewListProjectResponse(projects)
	_ = render.Render(w, r, util.NewServerResponse("Projects fetched successfully", resp, http.StatusOK))
}
