package handlers

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

func (h *Handler) InviteUserToOrganisation(w http.ResponseWriter, r *http.Request) {
	var newIV models.OrganisationInvite
	err := util.ReadJSON(r, &newIV)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	inviteService := &services.InviteUserService{
		Queue:        h.A.Queue,
		InviteRepo:   postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache),
		InviteeEmail: newIV.InviteeEmail,
		Role:         newIV.Role,
		User:         user,
		Organisation: org,
	}

	iv, err := inviteService.Run(r.Context())
	if err != nil {
		log.FromContext(r.Context()).Error(err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	res := models.UserInviteTokenResponse{Token: iv, User: user}
	_ = render.Render(w, r, util.NewServerResponse("invite created successfully", res, http.StatusCreated))
}

func (h *Handler) GetPendingOrganisationInvites(w http.ResponseWriter, r *http.Request) {
	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	invites, paginationData, err := postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache).LoadOrganisationsInvitesPaged(r.Context(), org.UID, datastore.InviteStatusPending, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load organisation invites")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invites fetched successfully",
		models.PagedResponse{Content: &invites, Pagination: &paginationData}, http.StatusOK))
}

func (h *Handler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	accepted, err := strconv.ParseBool(r.URL.Query().Get("accepted"))
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to process accepted url query")
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
		Queue:         h.A.Queue,
		InviteRepo:    postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache),
		UserRepo:      postgres.NewUserRepo(h.A.DB, h.A.Cache),
		OrgRepo:       postgres.NewOrgRepo(h.A.DB, h.A.Cache),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB, h.A.Cache),
		Token:         token,
		Accepted:      accepted,
		NewUser:       newUser,
	}

	err = prc.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite processed successfully", nil, http.StatusOK))
}

func (h *Handler) FindUserByInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	fub := &services.FindUserByInviteTokenService{
		Queue:      h.A.Queue,
		InviteRepo: postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache),
		OrgRepo:    postgres.NewOrgRepo(h.A.DB, h.A.Cache),
		UserRepo:   postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Token:      token,
	}

	user, iv, err := fub.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to find user by invite token")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	res := models.UserInviteTokenResponse{Token: iv, User: user}

	_ = render.Render(w, r, util.NewServerResponse("retrieved user", res, http.StatusOK))
}

func (h *Handler) ResendOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	rom := &services.ResendOrgMemberService{
		Queue:        h.A.Queue,
		InviteRepo:   postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache),
		InviteID:     chi.URLParam(r, "inviteID"),
		User:         user,
		Organisation: org,
	}

	_, err = rom.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to resend organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite resent successfully", nil, http.StatusOK))
}

func (h *Handler) CancelOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = h.A.Authz.Authorize(r.Context(), "organisation.manage", org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	cancelInvite := services.CancelOrgMemberService{
		Queue:      h.A.Queue,
		InviteRepo: postgres.NewOrgInviteRepo(h.A.DB, h.A.Cache),
		InviteID:   chi.URLParam(r, "inviteID"),
	}

	iv, err := cancelInvite.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Error("failed to cancel organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite cancelled successfully", iv, http.StatusOK))
}
