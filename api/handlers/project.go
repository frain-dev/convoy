package handlers

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/event_types"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

const errBillingRequired = "complete billing setup to create projects: add a subscription or payment method"

func createProjectService(h *Handler) *services.ProjectService {
	apiKeyRepo := api_keys.New(h.A.Logger, h.A.DB)
	// Must be the cache-invalidating repository: ProjectService.UpdateProject
	// persists config changes (meta events URL, signature versions, etc.) that
	// the API and dataplane read through the "projects:<id>" cache.
	projectRepo := h.projectRepo()
	eventRepo := events.New(h.A.Logger, h.A.DB)
	eventDeliveryRepo := event_deliveries.New(h.A.Logger, h.A.DB)
	eventTypesRepo := event_types.New(h.A.Logger, h.A.DB)

	return services.NewProjectService(
		apiKeyRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		eventTypesRepo,
		h.A.Licenser,
		h.A.Logger,
	)
}

// GetProject
//
//	@Summary		Retrieve a project
//	@Description	This endpoint fetches a project by its id
//	@Tags			Projects
//	@Id				GetProject
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Success		200			{object}	util.ServerResponse{data=models.ProjectResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID} [get]
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.ProjectResponse{Project: project}
	_ = render.Render(w, r, util.NewServerResponse("Project fetched successfully", resp, http.StatusOK))
}

func (h *Handler) GetProjectStatistics(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = h.projectRepo().FillProjectsStatistics(r.Context(), project)
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to count project statistics", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to count project statistics", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project Stats fetched successfully", project.Statistics, http.StatusOK))
}

// DeleteProject
//
//	@Summary		Delete a project
//	@Description	This endpoint deletes a project
//	@Tags			Projects
//	@Id				DeleteProject
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Success		200				{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,403,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID} [delete]
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionProjectManage), project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	err = h.projectRepo().DeleteProject(r.Context(), project.UID)
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to delete project", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete project", http.StatusBadRequest))
		return
	}

	h.A.Licenser.RemoveEnabledProject(project.UID)

	_ = render.Render(w, r, util.NewServerResponse("Project deleted successfully",
		nil, http.StatusOK))
}

// CreateProject
//
//	@Summary		Create a project
//	@Description	This endpoint creates a project. Authenticate with a personal API key or JWT and pass the organisation id as the orgID query parameter. The response includes the project and a one-time project API key.
//	@Tags			Projects
//	@Id				CreateProject
//	@Accept			json
//	@Produce		json
//	@Param			orgID				query		string					true	"Organisation ID"
//	@Param			project				body		models.CreateProject	true	"Project Details"
//	@Success		201					{object}	util.ServerResponse{data=models.CreateProjectResponse}
//	@Failure		400,401,402,403,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects [post]
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var newProject models.CreateProject
	err := util.ReadJSON(r, &newProject)
	if err != nil {
		h.A.Logger.Error("Failed to parse project creation request", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	member, err := h.retrieveMembership(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err := newProject.Validate(); err != nil {
		h.A.Logger.Error("Project creation validation failed", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	useOrgBilling := h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil
	skipLimitCheck := false
	if useOrgBilling {
		sub, err := h.A.BillingClient.GetSubscription(r.Context(), org.UID)
		if err != nil || !sub.Status || sub.Data.ID == "" {
			pms, pmErr := h.A.BillingClient.GetPaymentMethods(r.Context(), org.UID)
			if pmErr != nil || !pms.Status || len(pms.Data) == 0 {
				_ = render.Render(w, r, util.NewErrorResponse(errBillingRequired, http.StatusPaymentRequired))
				return
			}
		}
		limitDeps := services.OrgProjectLimitDeps{
			BillingClient: h.A.BillingClient,
			ProjectRepo:   h.projectRepo(),
			Cfg:           h.A.Cfg,
			Logger:        h.A.Logger,
		}
		ok, err := services.CheckOrganisationProjectLimit(r.Context(), org, limitDeps)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}
		if !ok {
			_ = render.Render(w, r, util.NewErrorResponse("organisation project limit reached", http.StatusPaymentRequired))
			return
		}
		skipLimitCheck = true
	}

	projectService := createProjectService(h)

	project, apiKey, err := projectService.CreateProject(r.Context(), &newProject, org, member, skipLimitCheck)
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

// UpdateProject
//
//	@Summary		Update a project
//	@Description	This endpoint updates a project's name, logo, and config
//	@Tags			Projects
//	@Id				UpdateProject
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string					true	"Project ID"
//	@Param			project			body		models.UpdateProject	true	"Project Details"
//	@Success		202				{object}	util.ServerResponse{data=models.ProjectResponse}
//	@Failure		400,401,403,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID} [put]
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateProject
	err := util.ReadJSON(r, &update)
	if err != nil {
		h.A.Logger.Error("Failed to parse project update request", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	p, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionProjectManage), p); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	if err := update.Validate(); err != nil {
		h.A.Logger.Error("Project update validation failed", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	projectService := createProjectService(h)

	project, err := projectService.UpdateProject(r.Context(), p, &update)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.ProjectResponse{Project: project}
	_ = render.Render(w, r, util.NewServerResponse("Project updated successfully", resp, http.StatusAccepted))
}

// GetProjects
//
//	@Summary		List all projects
//	@Description	This endpoint fetches projects for an organisation. Authenticate with a personal API key or JWT and pass the organisation id as the orgID query parameter.
//	@Tags			Projects
//	@Id				GetProjects
//	@Accept			json
//	@Produce		json
//	@Param			orgID		query		string	true	"Organisation ID"
//	@Success		200			{object}	util.ServerResponse{data=[]models.ProjectResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects [get]
func (h *Handler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projectsList, err := h.projectRepo().LoadProjects(r.Context(), filter)
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to load projects", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching projects", http.StatusBadRequest))
		return
	}

	resp := models.NewListProjectResponse(projectsList)
	_ = render.Render(w, r, util.NewServerResponse("Projects fetched successfully", resp, http.StatusOK))
}
