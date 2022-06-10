package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

// InviteUserToOrganisation
// @Summary Invite a user to join an organisation
// @Description This endpoint invites a user to join an organisation
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param invite body models.OrganisationInvite true "Organisation Invite Details"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /organisations/{orgID}/invite_user [post]
func (a *applicationHandler) InviteUserToOrganisation(w http.ResponseWriter, r *http.Request) {
	var newIV models.OrganisationInvite
	err := util.ReadJSON(r, &newIV)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	baseUrl := getBaseUrlFromContext(r.Context())
	user := getUserFromContext(r.Context())
	org := getOrganisationFromContext(r.Context())

	_, err = a.organisationInviteService.CreateOrganisationMemberInvite(r.Context(), &newIV, org, user, baseUrl)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("invite created successfully", nil, http.StatusCreated))
}

// ProcessOrganisationMemberInvite
// @Summary Accept or decline an organisation invite
// @Description This endpoint process a user's response to an organisation invite
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param token query string true "invite token"
// @Param accepted query string true "email"
// @Param user body models.User false "User Details"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /process_organisation_member_invite [post]
func (a *applicationHandler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	accepted, err := strconv.ParseBool(r.URL.Query().Get("accepted"))
	if err != nil {
		log.WithError(err).Error("failed to process accepted url query")
		_ = render.Render(w, r, newErrorResponse("badly formed 'accepted' query", http.StatusBadRequest))
		return
	}

	var newUser *models.User
	err = util.ReadJSON(r, &newUser)
	if err != nil && !errors.Is(err, util.ErrEmptyBody) {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = a.organisationInviteService.ProcessOrganisationMemberInvite(r.Context(), token, accepted, newUser)
	if err != nil {
		log.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, newServiceErrResponse(errors.New("")))
		return
	}

	_ = render.Render(w, r, newServerResponse("invite created successfully", nil, http.StatusOK))
}
