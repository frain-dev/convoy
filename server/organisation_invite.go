package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
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
// @Router /ui/organisations/{orgID}/invites [post]
func (a *ApplicationHandler) InviteUserToOrganisation(w http.ResponseWriter, r *http.Request) {
	var newIV models.OrganisationInvite
	err := util.ReadJSON(r, &newIV)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	baseUrl := m.GetHostFromContext(r.Context())
	user := m.GetUserFromContext(r.Context())
	org := m.GetOrganisationFromContext(r.Context())

	_, err = a.S.OrganisationInviteService.CreateOrganisationMemberInvite(r.Context(), &newIV, org, user, baseUrl)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite created successfully", nil, http.StatusCreated))
}

// GetPendingOrganisationInvites
// @Summary Fetch pending organisation invites
// @Description This endpoint fetches pending organisation invites
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Param orgID path string true "organisation id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/invites/pending [get]
func (a *ApplicationHandler) GetPendingOrganisationInvites(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())

	invites, paginationData, err := a.S.OrganisationInviteService.LoadOrganisationInvitesPaged(r.Context(), org, datastore.InviteStatusPending, pageable)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invites fetched successfully",
		pagedResponse{Content: &invites, Pagination: &paginationData}, http.StatusOK))
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
// @Router /ui/organisations/process_invite [post]
func (a *ApplicationHandler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	accepted, err := strconv.ParseBool(r.URL.Query().Get("accepted"))
	if err != nil {
		log.WithError(err).Error("failed to process accepted url query")
		_ = render.Render(w, r, util.NewErrorResponse("badly formed 'accepted' query", http.StatusBadRequest))
		return
	}

	var newUser *models.User
	err = util.ReadJSON(r, &newUser)
	if err != nil && !errors.Is(err, util.ErrEmptyBody) {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = a.S.OrganisationInviteService.ProcessOrganisationMemberInvite(r.Context(), token, accepted, newUser)
	if err != nil {
		log.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite processed successfully", nil, http.StatusOK))
}

// FindUserByInviteToken
// @Summary Find user by invite token
// @Description This endpoint finds a user by an invite token
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param token query string true "invite token"
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /users/token [get]
func (a *ApplicationHandler) FindUserByInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, iv, err := a.S.OrganisationInviteService.FindUserByInviteToken(r.Context(), token)
	if err != nil {
		log.WithError(err).Error("failed to find user by invite token")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	res := models.UserInviteTokenResponse{Token: iv, User: user}

	_ = render.Render(w, r, util.NewServerResponse("retrieved user", res, http.StatusOK))
}

// ResendOrganizationInvite
// @Summary resend organization invite
// @Description This endpoint resends the organization invite to a user
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param inviteID path string true "invite id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/invites/{inviteID}/resend [post]
func (a *ApplicationHandler) ResendOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	baseUrl := m.GetHostFromContext(r.Context())
	user := m.GetUserFromContext(r.Context())
	org := m.GetOrganisationFromContext(r.Context())

	_, err := a.S.OrganisationInviteService.ResendOrganisationMemberInvite(r.Context(), chi.URLParam(r, "inviteID"), org, user, baseUrl)
	if err != nil {
		log.WithError(err).Error("failed to resend organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite resent successfully", nil, http.StatusOK))
}

// CancelOrganizationInvite
// @Summary cancel organization invite
// @Description This endpoint cancels an organization invite
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param orgID path string true "organisation id"
// @Param inviteID path string true "invite id"
// @Success 200 {object} serverResponse{data=datastore.OrganisationInvite}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/invites/{inviteID}/cancel [post]
func (a *ApplicationHandler) CancelOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	iv, err := a.S.OrganisationInviteService.CancelOrganisationMemberInvite(r.Context(), chi.URLParam(r, "inviteID"))
	if err != nil {
		log.WithError(err).Error("failed to cancel organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite cancelled successfully", iv, http.StatusOK))
}
