package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createOrganisationMemberService(a *ApplicationHandler) *services.OrganisationMemberService {
	orgMemberRepo := mongo.NewOrgMemberRepo(a.A.Store)

	return services.NewOrganisationMemberService(orgMemberRepo)
}

func (a *ApplicationHandler) GetOrganisationMembers(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	org := m.GetOrganisationFromContext(r.Context())
	orgMemberService := createOrganisationMemberService(a)

	members, paginationData, err := orgMemberService.LoadOrganisationMembersPaged(r.Context(), org, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation members fetched successfully",
		pagedResponse{Content: &members, Pagination: &paginationData}, http.StatusOK))
}

func (a *ApplicationHandler) GetOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())
	orgMemberService := createOrganisationMemberService(a)

	member, err := orgMemberService.FindOrganisationMemberByID(r.Context(), org, memberID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member fetched successfully", member, http.StatusOK))
}

func (a *ApplicationHandler) UpdateOrganisationMember(w http.ResponseWriter, r *http.Request) {
	var roleUpdate models.UpdateOrganisationMember
	err := util.ReadJSON(r, &roleUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())
	orgMemberService := createOrganisationMemberService(a)

	member, err := orgMemberService.FindOrganisationMemberByID(r.Context(), org, memberID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	organisationMember, err := orgMemberService.UpdateOrganisationMember(r.Context(), member, &roleUpdate.Role)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member updated successfully", organisationMember, http.StatusAccepted))
}

func (a *ApplicationHandler) DeleteOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())
	orgMemberService := createOrganisationMemberService(a)

	err := orgMemberService.DeleteOrganisationMember(r.Context(), memberID, org)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member deleted successfully", nil, http.StatusOK))
}
