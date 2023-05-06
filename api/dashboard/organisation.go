package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createOrganisationService(a *DashboardHandler) *services.OrganisationService {
	orgRepo := postgres.NewOrgRepo(a.A.DB)
	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)

	return services.NewOrganisationService(orgRepo, orgMemberRepo)
}

func (a *DashboardHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation fetched successfully", org, http.StatusOK))
}

func (a *DashboardHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
	pageable := m.GetPageableFromContext(r.Context())
	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgService := createOrganisationService(a)

	organisations, paginationData, err := orgService.LoadUserOrganisationsPaged(r.Context(), user, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		pagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var newOrg models.Organisation
	err := util.ReadJSON(r, &newOrg)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgService := createOrganisationService(a)
	organisation, err := orgService.CreateOrganisation(r.Context(), &newOrg, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation created successfully", organisation, http.StatusCreated))
}

func (a *DashboardHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgUpdate models.Organisation
	err := util.ReadJSON(r, &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	orgService := createOrganisationService(a)

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	org, err = orgService.UpdateOrganisation(r.Context(), org, &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", org, http.StatusAccepted))
}

func (a *DashboardHandler) DeleteOrganisation(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	orgService := createOrganisationService(a)
	err = orgService.DeleteOrganisation(r.Context(), org.UID)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete organisation")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation deleted successfully", nil, http.StatusOK))
}
