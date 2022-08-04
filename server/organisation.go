package server

import (
	"net/http"

	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

// GetOrganisation
// @Summary Get an organisation
// @Description This endpoint fetches an organisation by its id
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} util.ServerResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID} [get]
func (a *ApplicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, util.NewServerResponse("Organisation fetched successfully",
		m.GetOrganisationFromContext(r.Context()), http.StatusOK))
}

// GetOrganisationsPaged
// @Summary Get organisations
// @Description This endpoint fetches multiple organisations
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Organisation}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations [get]
func (a *ApplicationHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { //TODO: change to GetUserOrganisationsPaged
	pageable := m.GetPageableFromContext(r.Context())
	user := m.GetUserFromContext(r.Context())

	organisations, paginationData, err := a.S.OrganisationService.LoadUserOrganisationsPaged(r.Context(), user, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		pagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

// CreateOrganisation
// @Summary Create an organisation
// @Description This endpoint creates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations [post]
func (a *ApplicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var newOrg models.Organisation
	err := util.ReadJSON(r, &newOrg)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user := m.GetUserFromContext(r.Context())

	organisation, err := a.S.OrganisationService.CreateOrganisation(r.Context(), &newOrg, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation created successfully", organisation, http.StatusCreated))
}

// UpdateOrganisation
// @Summary Update an organisation
// @Description This endpoint updates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID} [put]
func (a *ApplicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgUpdate models.Organisation
	err := util.ReadJSON(r, &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org, err := a.S.OrganisationService.UpdateOrganisation(r.Context(), m.GetOrganisationFromContext(r.Context()), &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", org, http.StatusAccepted))
}

// DeleteOrganisation
// @Summary Delete organisation
// @Description This endpoint deletes an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID} [delete]
func (a *ApplicationHandler) DeleteOrganisation(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())
	err := a.S.OrganisationService.DeleteOrganisation(r.Context(), org.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete organisation")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation deleted successfully", nil, http.StatusOK))
}
