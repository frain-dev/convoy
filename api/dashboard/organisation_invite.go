package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func (a *DashboardHandler) InviteUserToOrganisation(w http.ResponseWriter, r *http.Request) {
	var newIV models.OrganisationInvite
	err := util.ReadJSON(r, &newIV)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	inviteService := &services.InviteUserService{
		Queue:        a.A.Queue,
		InviteRepo:   postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache),
		InviteeEmail: newIV.InviteeEmail,
		Role:         newIV.Role,
		User:         user,
		Organisation: org,
	}

	_, err = inviteService.Run(r.Context())
	if err != nil {
		log.FromContext(r.Context()).Error(err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite created successfully", nil, http.StatusCreated))
}

func (a *DashboardHandler) GetPendingOrganisationInvites(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	invites, paginationData, err := postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache).LoadOrganisationsInvitesPaged(r.Context(), org.UID, datastore.InviteStatusPending, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load organisation invites")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invites fetched successfully",
		pagedResponse{Content: &invites, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	accepted, err := strconv.ParseBool(r.URL.Query().Get("accepted"))
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to process accepted url query")
		_ = render.Render(w, r, util.NewErrorResponse("badly formed 'accepted' query", http.StatusBadRequest))
		return
	}

	var newUser *models.User
	err = util.ReadJSON(r, &newUser)
	if err != nil && !errors.Is(err, util.ErrEmptyBody) {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	prc := services.ProcessInviteService{
		Queue:         a.A.Queue,
		InviteRepo:    postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache),
		UserRepo:      postgres.NewUserRepo(a.A.DB, a.A.Cache),
		OrgRepo:       postgres.NewOrgRepo(a.A.DB, a.A.Cache),
		OrgMemberRepo: postgres.NewOrgMemberRepo(a.A.DB, a.A.Cache),
		Token:         token,
		Accepted:      accepted,
		NewUser:       newUser,
	}

	err = prc.Run(r.Context())
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite processed successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) FindUserByInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	fub := &services.FindUserByInviteTokenService{
		Queue:      a.A.Queue,
		InviteRepo: postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache),
		OrgRepo:    postgres.NewOrgRepo(a.A.DB, a.A.Cache),
		UserRepo:   postgres.NewUserRepo(a.A.DB, a.A.Cache),
		Token:      token,
	}

	user, iv, err := fub.Run(r.Context())
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to find user by invite token")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	res := models.UserInviteTokenResponse{Token: iv, User: user}

	_ = render.Render(w, r, util.NewServerResponse("retrieved user", res, http.StatusOK))
}

func (a *DashboardHandler) ResendOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	rom := &services.ResendOrgMemberService{
		Queue:        a.A.Queue,
		InviteRepo:   postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache),
		InviteID:     chi.URLParam(r, "inviteID"),
		User:         user,
		Organisation: org,
	}

	_, err = rom.Run(r.Context())
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to resend organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite resent successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) CancelOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	cancelInvite := services.CancelOrgMemberService{
		Queue:      a.A.Queue,
		InviteRepo: postgres.NewOrgInviteRepo(a.A.DB, a.A.Cache),
		InviteID:   chi.URLParam(r, "inviteID"),
	}

	iv, err := cancelInvite.Run(r.Context())
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to cancel organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite cancelled successfully", iv, http.StatusOK))
}
