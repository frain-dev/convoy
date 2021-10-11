package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetOrganisation
// @Summary Get an organisation
// @Description This endpoint fetches an organisation by it's id
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [get]
func (a *applicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Organisation fetched successfully",
		*getOrganisationFromContext(r.Context()), http.StatusOK))
}

// CreateOrganisation
// @Summary Create an organisation
// @Description This endpoint creates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations [post]
func (a *applicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {

	var newOrg models.Organisation
	err := json.NewDecoder(r.Body).Decode(&newOrg)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	orgName := newOrg.Name
	if util.IsStringEmpty(orgName) {
		_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
		return
	}

	org := &convoy.Organisation{
		UID:            uuid.New().String(),
		OrgName:        orgName,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.orgRepo.CreateOrganisation(r.Context(), org)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating organisation", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisation created successfully", org, http.StatusCreated))
}

// UpdateOrganisation
// @Summary Update an organisation
// @Description This endpoint updates an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param organisation body models.Organisation true "Organisation Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations/{orgID} [put]
func (a *applicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {

	var update models.Organisation
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	orgName := update.Name
	if util.IsStringEmpty(orgName) {
		_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
		return
	}

	orgId := chi.URLParam(r, "orgID")

	org, err := a.orgRepo.FetchOrganisationByID(r.Context(), orgId)
	if err != nil {

		msg := "an error occurred while retrieving organisation details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, convoy.ErrOrganisationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	org.OrgName = orgName
	err = a.orgRepo.UpdateOrganisation(r.Context(), org)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while updating organisation", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisation updated successfully", org, http.StatusAccepted))
}

// GetOrganisations
// @Summary Get organisations
// @Description This endpoint fetches organisations
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /organisations [get]
func (a *applicationHandler) GetOrganisations(w http.ResponseWriter, r *http.Request) {

	orgs, err := a.orgRepo.LoadOrganisations(r.Context())
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching organisations", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("Organisations fetched successfully", orgs, http.StatusOK))
}
