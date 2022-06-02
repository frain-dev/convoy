package server

import (
	"errors"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

// InviteUserToOrganisation
// @Summary Get an organisation
// @Description This endpoint invites a user to join an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations/{orgID}/invite_user [get]
func (a *applicationHandler) InviteUserToOrganisation(w http.ResponseWriter, r *http.Request) {
	var newIV models.OrganisationInvite
	err := util.ReadJSON(r, &newIV)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	org := getOrganisationFromContext(r.Context())
	iv, err := a.organisationInviteService.CreateOrganisationMemberInvite(r.Context(), org, &newIV)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("invite created successfully", iv, http.StatusCreated))
}

// ProcessOrganisationMemberInvite
// @Summary Get organisations
// @Description This endpoint fetches multiple organisations
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Organisation}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /process_organisation_member_invite [get]
func (a *applicationHandler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	email := r.URL.Query().Get("email")
	accepted, err := strconv.ParseBool(r.URL.Query().Get("accepted"))
	if err != nil {
		log.WithError(err).Error("failed to load process accepted query")
		_ = render.Render(w, r, newErrorResponse("badly formed 'accepted' query", http.StatusBadRequest))
		return
	}

	var newUser *models.User
	err = util.ReadJSON(r, &newUser)
	if err != nil && !errors.Is(err, util.ErrEmptyBody) {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = a.organisationInviteService.AcceptOrganisationMemberInvite(r.Context(), token, email, accepted, newUser)
	if err != nil {
		log.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, newServiceErrResponse(errors.New("")))
		return
	}

	_ = render.Render(w, r, newServerResponse("invite created successfully", nil, http.StatusOK))
}
