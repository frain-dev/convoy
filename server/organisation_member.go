package server

import (
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	"net/http"

	m "github.com/frain-dev/convoy/pkg/middleware"
)

// GetOrganisationMembers
// @Summary Get organisation members
// @Description This endpoint fetches an organisation's members
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.OrganisationMember}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/members [get]
func (a *applicationHandler) GetOrganisationMembers(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	org := m.GetOrganisationFromContext(r.Context())

	members, paginationData, err := a.organisationMemberService.LoadOrganisationMembersPaged(r.Context(), org, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation members fetched successfully",
		pagedResponse{Content: &members, Pagination: &paginationData}, http.StatusOK))
}

// GetOrganisationMember
// @Summary Get organisation member
// @Description This endpoint fetches an organisation's member
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param memberID path string true "organisation member id"
// @Success 200 {object} serverResponse{data=datastore.OrganisationMember}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/members/{memberID} [get]
func (a *applicationHandler) GetOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())

	member, err := a.organisationMemberService.FindOrganisationMemberByID(r.Context(), org, memberID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member fetched successfully", member, http.StatusOK))
}

// UpdateOrganisationMember
// @Summary Update an organisation's member
// @Description This endpoint updates an organisation's member
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param memberID path string true "organisation member id"
// @Param organisation_member body models.UpdateOrganisationMember true "Organisation member Details"
// @Success 200 {object} serverResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/members/{memberID} [put]
func (a *applicationHandler) UpdateOrganisationMember(w http.ResponseWriter, r *http.Request) {
	var roleUpdate models.UpdateOrganisationMember
	err := util.ReadJSON(r, &roleUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())

	member, err := a.organisationMemberService.FindOrganisationMemberByID(r.Context(), org, memberID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	organisationMember, err := a.organisationMemberService.UpdateOrganisationMember(r.Context(), member, &roleUpdate.Role)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member updated successfully", organisationMember, http.StatusAccepted))
}

// DeleteOrganisationMember
// @Summary Delete an organisation's member
// @Description This endpoint deletes an organisation's member
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param memberID path string true "organisation member id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/members/{memberID} [delete]
func (a *applicationHandler) DeleteOrganisationMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberID")
	org := m.GetOrganisationFromContext(r.Context())

	err := a.organisationMemberService.DeleteOrganisationMember(r.Context(), memberID, org)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation member deleted successfully", nil, http.StatusOK))
}
