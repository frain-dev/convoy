package public

import (
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createProjectService(a *PublicHandler) *services.ProjectService {
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
// @Summary Retrieve a project
// @Description This endpoint fetches a project by its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID} [get]
func (a *PublicHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", project, http.StatusOK))
}

func (a *PublicHandler) GetProjectStatistics(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
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

// DeleteProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Delete a project
// @Description This endpoint deletes a project using its id
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID} [delete]
func (a *PublicHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
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

// CreateProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Create a project
// @Description This endpoint creates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param orgID query string true "Organisation id"
// @Param project body models.Project true "Project Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects [post]
func (a *PublicHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveHeadlessOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	member, err := a.retrieveHeadlessMembership(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	var newProject models.Project
	err = util.ReadJSON(r, &newProject)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
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

// UpdateProject - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Update a project
// @Description This endpoint updates a project
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param project body models.Project true "Project Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Project}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID} [put]
func (a *PublicHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateProject
	err := util.ReadJSON(r, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	projectService := createProjectService(a)
	project, err = projectService.UpdateProject(r.Context(), project, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project updated successfully", project, http.StatusAccepted))
}

// GetProjects - this is a duplicate annotation for the api/v1 route of this handler
// @Summary List all projects
// @Description This endpoint fetches projects
// @Tags Projects
// @Accept  json
// @Produce  json
// @Param name query string false "Project name"
// @Param orgID query string true "organisation id"
// @Success 200 {object} util.ServerResponse{data=[]datastore.Project}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects [get]
func (a *PublicHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveHeadlessOrganisation(r)
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

func (a *PublicHandler) retrieveHeadlessOrganisation(r *http.Request) (*datastore.Organisation, error) {
	orgID := r.URL.Query().Get("orgID")
	orgRepo := postgres.NewOrgRepo(a.A.DB)

	return orgRepo.FetchOrganisationByID(r.Context(), orgID)
}

func (a *PublicHandler) retrieveHeadlessMembership(r *http.Request) (*datastore.OrganisationMember, error) {
	org, err := a.retrieveHeadlessOrganisation(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	user, err := a.retrieveUser(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)
	return orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
}
