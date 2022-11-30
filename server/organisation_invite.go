package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func CreateOrganisationInviteService(a *ApplicationHandler) *services.OrganisationInviteService {
	userRepo := mongo.NewUserRepo(a.A.Store)
	orgRepo := mongo.NewOrgRepo(a.A.Store)
	orgMemberRepo := mongo.NewOrgMemberRepo(a.A.Store)
	orgInviteRepo := mongo.NewOrgInviteRepo(a.A.Store)

	return services.NewOrganisationInviteService(
		orgRepo, userRepo, orgMemberRepo,
		orgInviteRepo, a.A.Queue,
	)
}

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

	organisationInviteService := CreateOrganisationInviteService(a)
	_, err = organisationInviteService.CreateOrganisationMemberInvite(r.Context(), &newIV, org, user, baseUrl)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite created successfully", nil, http.StatusCreated))
}

func (a *ApplicationHandler) GetPendingOrganisationInvites(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())
	organisationInviteService := CreateOrganisationInviteService(a)

	invites, paginationData, err := organisationInviteService.LoadOrganisationInvitesPaged(r.Context(), org, datastore.InviteStatusPending, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to create organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invites fetched successfully",
		pagedResponse{Content: &invites, Pagination: &paginationData}, http.StatusOK))
}

func (a *ApplicationHandler) ProcessOrganisationMemberInvite(w http.ResponseWriter, r *http.Request) {
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
	organisationInviteService := CreateOrganisationInviteService(a)

	err = organisationInviteService.ProcessOrganisationMemberInvite(r.Context(), token, accepted, newUser)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to process organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite processed successfully", nil, http.StatusOK))
}

func (a *ApplicationHandler) FindUserByInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	organisationInviteService := CreateOrganisationInviteService(a)

	user, iv, err := organisationInviteService.FindUserByInviteToken(r.Context(), token)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to find user by invite token")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	res := models.UserInviteTokenResponse{Token: iv, User: user}

	_ = render.Render(w, r, util.NewServerResponse("retrieved user", res, http.StatusOK))
}

func (a *ApplicationHandler) ResendOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	baseUrl := m.GetHostFromContext(r.Context())
	user := m.GetUserFromContext(r.Context())
	org := m.GetOrganisationFromContext(r.Context())
	organisationInviteService := CreateOrganisationInviteService(a)

	_, err := organisationInviteService.ResendOrganisationMemberInvite(r.Context(), chi.URLParam(r, "inviteID"), org, user, baseUrl)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to resend organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite resent successfully", nil, http.StatusOK))
}

func (a *ApplicationHandler) CancelOrganizationInvite(w http.ResponseWriter, r *http.Request) {
	organisationInviteService := CreateOrganisationInviteService(a)

	iv, err := organisationInviteService.CancelOrganisationMemberInvite(r.Context(), chi.URLParam(r, "inviteID"))
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to cancel organisation member invite")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("invite cancelled successfully", iv, http.StatusOK))
}
