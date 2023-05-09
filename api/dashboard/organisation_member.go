package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createOrganisationMemberService(a *DashboardHandler) *services.OrganisationMemberService {
	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)

	return services.NewOrganisationMemberService(orgMemberRepo)
}

func (a *DashboardHandler) GetOrganisationMembers(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	userID := r.URL.Query().Get("userID")

	members, paginationData, err := postgres.NewOrgMemberRepo(a.A.DB).LoadOrganisationMembersPaged(r.Context(), org.UID, userID, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch organisation members")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation members fetched successfully",
		pagedResponse{Content: &members, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) GetOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	member, err := postgres.NewOrgMemberRepo(a.A.DB).FetchOrganisationMemberByID(r.Context(), memberID, org.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find organisation member by id")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member fetched successfully", member, http.StatusOK))
}

func (a *DashboardHandler) UpdateOrganisationMember(w http.ResponseWriter, r *http.Request) {
	var roleUpdate models.UpdateOrganisationMember
	err := util.ReadJSON(r, &roleUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	memberID := chi.URLParam(r, "memberID")
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	member, err := postgres.NewOrgMemberRepo(a.A.DB).FetchOrganisationMemberByID(r.Context(), memberID, org.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find organisation member by id")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	orgMemberService := createOrganisationMemberService(a)
	organisationMember, err := orgMemberService.UpdateOrganisationMember(r.Context(), member, &roleUpdate.Role)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member updated successfully", organisationMember, http.StatusAccepted))
}

func (a *DashboardHandler) DeleteOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	orgMemberService := createOrganisationMemberService(a)
	err = orgMemberService.DeleteOrganisationMember(r.Context(), memberID, org)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member deleted successfully", nil, http.StatusOK))
}
