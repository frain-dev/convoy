package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.RegisterUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		h.A.Logger.Errorf("Failed to parse user registration request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := newUser.Validate(); err != nil {
		h.A.Logger.Errorf("User registration validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	config, err := config.Get()
	if err != nil {
		h.A.Logger.Errorf("Failed to get configuration: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Service temporarily unavailable", http.StatusInternalServerError))
		return
	}

	rs := services.NewRegisterUserService(
		users.New(h.A.Logger, h.A.DB),
		organisations.New(h.A.Logger, h.A.DB),
		organisation_members.New(h.A.Logger, h.A.DB),
		h.A.Queue,
		jwt.NewJwt(&config.Auth.Jwt, h.A.Cache),
		h.A.ConfigRepo,
		h.A.Licenser,
		baseUrl,
		&newUser,
		h.A.Logger,
	)

	user, token, err := rs.Run(r.Context())
	if err != nil {
		if errors.Is(err, datastore.ErrSignupDisabled) {
			_ = render.Render(w, r, util.NewErrorResponse(datastore.ErrSignupDisabled.Error(), http.StatusForbidden))
			return
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Registration successful", u, http.StatusCreated))

	// Fail-open: signup already succeeded; onboarding welcome must not fail the response.
	// Fire-and-forget: Overwatch ensure+welcome can take up to two client timeouts;
	// do not hold the registration handler. Detach from r.Context so client
	// disconnect / handler return cannot cancel the work.
	welcomeCtx := context.WithoutCancel(r.Context())
	go h.enqueueCloudOnboardingWelcome(welcomeCtx, user)
}

// enqueueCloudOnboardingWelcome ensures the billing org exists and asks Overwatch
// to send the Motunrayo welcome email. Cloud-only; errors are logged only.
func (h *Handler) enqueueCloudOnboardingWelcome(ctx context.Context, user *datastore.User) {
	if user == nil || !h.A.Cfg.UsesOrgBilling() || h.A.BillingClient == nil {
		return
	}

	cfg, err := config.Get()
	if err != nil {
		h.A.Logger.Warn("onboarding welcome skipped: config unavailable", "error", err)
		return
	}
	// Same host preference as CreateOrganisationService so welcome ensure matches
	// the billing org host written during signup (OrganisationHost overrides Host).
	hostForBilling := strings.TrimSpace(cfg.Host)
	if strings.TrimSpace(cfg.Billing.OrganisationHost) != "" {
		hostForBilling = strings.TrimSpace(cfg.Billing.OrganisationHost)
	}
	if hostForBilling == "" {
		h.A.Logger.Warn("onboarding welcome skipped: host unavailable")
		return
	}

	orgMemberRepo := organisation_members.New(h.A.Logger, h.A.DB)
	orgs, _, err := orgMemberRepo.LoadUserOrganisationsPaged(ctx, user.UID, datastore.Pageable{
		PerPage:    1,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	if err != nil || len(orgs) == 0 {
		h.A.Logger.Warn("onboarding welcome skipped: no org for user", "user_id", user.UID, "error", err)
		return
	}
	org := orgs[0]

	_, createErr := h.A.BillingClient.CreateOrganisation(ctx, billing.BillingOrganisation{
		Name:         org.Name,
		ExternalID:   org.UID,
		BillingEmail: user.Email,
		Host:         hostForBilling,
	})
	if createErr != nil {
		h.A.Logger.Warn("onboarding welcome: ensure billing org failed", "org_id", org.UID, "error", createErr)
		// Continue: org may already exist; welcome still worth trying.
	}

	_, welcomeErr := h.A.BillingClient.EnqueueOnboardingWelcome(ctx, org.UID, billing.OnboardingWelcomeRequest{
		FirstName: user.FirstName,
		Track:     "cloud",
	})
	if welcomeErr != nil {
		h.A.Logger.Warn("onboarding welcome enqueue failed", "org_id", org.UID, "error", welcomeErr)
	}
}

func (h *Handler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	rs := services.ResendEmailVerificationTokenService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Queue:    h.A.Queue,
		BaseURL:  baseUrl,
		User:     user,
		Logger:   h.A.Logger,
	}

	err = rs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Verification email resent successfully", nil, http.StatusOK))
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("User fetched successfully", userResponse, http.StatusOK))
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var userUpdate models.UpdateUser
	err := util.ReadJSON(r, &userUpdate)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse user update request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := userUpdate.Validate(); err != nil {
		h.A.Logger.Errorf("User update validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := services.UpdateUserService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Queue:    h.A.Queue,
		BaseURL:  baseUrl,
		Data:     &userUpdate,
		User:     user,
		Logger:   h.A.Logger,
	}

	user, err = u.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("User updated successfully", userResponse, http.StatusOK))
}

func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var updatePassword models.UpdatePassword
	err := util.ReadJSON(r, &updatePassword)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse password update request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := updatePassword.Validate(); err != nil {
		h.A.Logger.Errorf("Password update validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	up := services.UpdatePasswordService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Data:     &updatePassword,
		User:     user,
		Logger:   h.A.Logger,
	}

	user, err = up.Run(r.Context())
	if err != nil {
		msg := "unable to complete password change request"

		h.A.Logger.Errorf("%s: %v", msg, err)

		_ = render.Render(w, r, util.NewServiceErrResponse(errors.New(msg)))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("Password updated successfully", userResponse, http.StatusOK))
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var forgotPassword models.ForgotPassword
	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = util.ReadJSON(r, &forgotPassword)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse forgot password request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := forgotPassword.Validate(); err != nil {
		h.A.Logger.Errorf("Forgot password validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	gp := services.GeneratePasswordResetTokenService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Queue:    h.A.Queue,
		BaseURL:  baseUrl,
		Data:     &forgotPassword,
		Logger:   h.A.Logger,
	}

	_ = gp.Run(r.Context())
	_ = render.Render(w, r, util.NewServerResponse("if your email is registered on the platform, please check the email we have sent you to verify your account", nil, http.StatusOK))
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ve := services.VerifyEmailService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Token:    r.URL.Query().Get("token"),
		Logger:   h.A.Logger,
	}

	err := ve.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Email has been verified successfully", nil, http.StatusOK))
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var resetPassword models.ResetPassword
	err := util.ReadJSON(r, &resetPassword)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse reset password request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := resetPassword.Validate(); err != nil {
		h.A.Logger.Errorf("Reset password validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	rs := services.ResetPasswordService{
		UserRepo: users.New(h.A.Logger, h.A.DB),
		Token:    token,
		Data:     &resetPassword,
		Logger:   h.A.Logger,
	}

	user, err := rs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to reset password", http.StatusBadRequest))
		return
	}

	userResponse := models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("Password reset successful", userResponse, http.StatusOK))
}

func getUser(r *http.Request) (*datastore.User, bool) {
	authUser := m.GetAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)

	return user, ok
}
