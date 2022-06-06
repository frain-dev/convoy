package server

import (
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// GetOrganisation
// @Summary Get an organisation
// @Description This endpoint fetches an organisation by its id
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [get]
func (a *applicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation fetched successfully",
		getOrganisationFromContext(r.Context()), http.StatusOK))
}

// GetOrganisationsPaged
// @Summary Get organisations
// @Description This endpoint fetches multiple organisations
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Organisation}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations [get]
func (a *applicationHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())

	organisations, paginationData, err := a.organisationService.LoadOrganisationsPaged(r.Context(), pageable)
	if err != nil {
		log.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisations fetched successfully",
		pagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

// CreateOrganisation
// @Summary Create an organisation
// @Description This endpoint creates an organisation
// @Tags Application
// @Accept  json
// @Produce  json
// @Param application body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations [post]
func (a *applicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var newOrg models.Organisation
	err := util.ReadJSON(r, &newOrg)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	organisation, err := a.organisationService.CreateOrganisation(r.Context(), &newOrg)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisation created successfully", organisation, http.StatusCreated))
}

// UpdateOrganisation
// @Summary Update an organisation
// @Description This endpoint updates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param application body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse{data=datastore.Organisation}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [put]
func (a *applicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgUpdate models.Organisation
	err := util.ReadJSON(r, &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org, err := a.organisationService.UpdateOrganisation(r.Context(), getOrganisationFromContext(r.Context()), &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("App updated successfully", org, http.StatusAccepted))
}

// DeleteOrganisation
// @Summary Delete organisation
// @Description This endpoint deletes an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [put]
func (a *applicationHandler) DeleteOrganisation(w http.ResponseWriter, r *http.Request) {
	org := getOrganisationFromContext(r.Context())
	err := a.organisationService.DeleteOrganisation(r.Context(), org.UID)
	if err != nil {
		log.WithError(err).Error("failed to delete organisation")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisation deleted successfully", nil, http.StatusOK))
}
