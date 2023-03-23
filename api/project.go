package api

import (
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createProjectService(a *ApplicationHandler) *services.ProjectService {
	apiKeyRepo := postgres.NewAPIKeyRepo(a.A.DB)
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	eventRepo := postgres.NewEventRepo(a.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)

	return services.NewProjectService(
		apiKeyRepo, projectRepo, eventRepo,
		eventDeliveryRepo, a.A.Limiter, a.A.Cache,
	)
}

// GetProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Get a project
// @Description This endpoint fetches a project by its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [get]
func _() {}

func (a *ApplicationHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	projectService := createProjectService(a)

	err := projectService.FillProjectStatistics(r.Context(), []*datastore.Project{project})
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully",
		project, http.StatusOK))
}

// DeleteProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Delete a project
// @Description This endpoint deletes a project using its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [delete]
func _() {}

func (a *ApplicationHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project := m.GetProjectFromContext(r.Context())
	projectService := createProjectService(a)

	//opts := &policies.ProjectPolicyOpts{
	//	OrganisationRepo:       mongo.NewOrgRepo(a.A.DB),
	//	OrganisationMemberRepo: mongo.NewOrgMemberRepo(a.A.DB),
	//}
	//gp := policies.NewProjectPolicy(opts)
	//if err := gp.Delete(r.Context(), group); err != nil {
	//	_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
	//	return
	//}

	err := projectService.DeleteProject(r.Context(), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project deleted successfully",
		nil, http.StatusOK))
}

// CreateProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Create a project
// @Description This endpoint creates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param orgID query string true "Organisation id"
// @Param project body models.Project true "Project Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects [post]
func _() {}

func (a *ApplicationHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var newProject models.Project
	err := util.ReadJSON(r, &newProject)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org := m.GetOrganisationFromContext(r.Context())
	member := m.GetOrganisationMemberFromContext(r.Context())
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

// UpdateProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Update a project
// @Description This endpoint updates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param project body models.Project true "Project Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID} [put]
func _() {}

func (a *ApplicationHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateProject
	err := util.ReadJSON(r, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	p := m.GetProjectFromContext(r.Context())
	projectService := createProjectService(a)

	project, err := projectService.UpdateProject(r.Context(), p, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project updated successfully", project, http.StatusAccepted))
}

// GetProjects - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Get projects
// @Description This endpoint fetches projects
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param name query string false "Project name"
// @Param orgID query string true "organisation id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Project}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects [get]
func _() {}

func (a *ApplicationHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projectService := createProjectService(a)

	projects, err := projectService.GetProjects(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Projects fetched successfully", projects, http.StatusOK))
}
