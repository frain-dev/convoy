package handlers

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

const errBillingRequired = "complete billing setup to create projects: add a subscription or payment method"

func createProjectService(h *Handler) (*services.ProjectService, error) {
	apiKeyRepo := api_keys.New(h.A.Logger, h.A.DB)
	projectRepo := projects.New(h.A.Logger, h.A.DB)
	eventRepo := postgres.NewEventRepo(h.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(h.A.DB)
	eventTypesRepo := postgres.NewEventTypesRepo(h.A.DB)

	projectService, err := services.NewProjectService(apiKeyRepo, projectRepo, eventRepo, eventDeliveryRepo, h.A.Licenser, eventTypesRepo)
	if err != nil {
		return nil, err
	}

	return projectService, nil
}

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

	err = projects.New(h.A.Logger, h.A.DB).FillProjectsStatistics(r.Context(), project)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to count project statistics")
		_ = render.Render(w, r, util.NewErrorResponse("failed to count project statistics", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Project Stats fetched successfully", project.Statistics, http.StatusOK))
}

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

	err = projects.New(h.A.Logger, h.A.DB).DeleteProject(r.Context(), project.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete project")
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete project", http.StatusBadRequest))
		return
	}

	h.A.Licenser.RemoveEnabledProject(project.UID)

	_ = render.Render(w, r, util.NewServerResponse("Project deleted successfully",
		nil, http.StatusOK))
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var newProject models.CreateProject
	err := util.ReadJSON(r, &newProject)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Failed to parse project creation request: %v", err)
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
		h.A.Logger.WithError(err).Errorf("Project creation validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	billingEnabled := h.A.Cfg.Billing.Enabled && h.A.BillingClient != nil
	skipLimitCheck := false
	if billingEnabled {
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
			ProjectRepo:   projects.New(h.A.Logger, h.A.DB),
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

	projectService, err := createProjectService(h)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

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

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateProject
	err := util.ReadJSON(r, &update)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Failed to parse project update request: %v", err)
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
		h.A.Logger.WithError(err).Errorf("Project update validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	projectService, err := createProjectService(h)
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

func (h *Handler) GetProjects(w http.ResponseWriter, r *http.Request) {
	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projectsList, err := projects.New(h.A.Logger, h.A.DB).LoadProjects(r.Context(), filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load projects")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching projects", http.StatusBadRequest))
		return
	}

	resp := models.NewListProjectResponse(projectsList)
	_ = render.Render(w, r, util.NewServerResponse("Projects fetched successfully", resp, http.StatusOK))
}
