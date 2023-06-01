package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createUserService(a *DashboardHandler) *services.UserService {
	userRepo := postgres.NewUserRepo(a.A.DB)
	configService := createConfigService(a)
	orgService := createOrganisationService(a)

	return services.NewUserService(
		userRepo, a.A.Cache, a.A.Queue,
		configService, orgService,
	)
}

func (a *DashboardHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.RegisterUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := newUser.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	userService := createUserService(a)
	user, token, err := userService.RegisterUser(r.Context(), baseUrl, &newUser)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Registration successful", u, http.StatusCreated))
}

func (a *DashboardHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	userService := createUserService(a)
	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = userService.ResendEmailVerificationToken(r.Context(), baseUrl, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Verification email resent successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("User fetched successfully", userResponse, http.StatusOK))
}

func (a *DashboardHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var userUpdate models.UpdateUser
	err := util.ReadJSON(r, &userUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := userUpdate.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	userService := createUserService(a)
	user, err = userService.UpdateUser(r.Context(), &userUpdate, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("User updated successfully", userResponse, http.StatusOK))
}

func (a *DashboardHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var updatePassword models.UpdatePassword
	err := util.ReadJSON(r, &updatePassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := updatePassword.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	userService := createUserService(a)
	user, err = userService.UpdatePassword(r.Context(), &updatePassword, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	userResponse := &models.UserResponse{User: user}
	_ = render.Render(w, r, util.NewServerResponse("Password updated successfully", userResponse, http.StatusOK))
}

func (a *DashboardHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var forgotPassword models.ForgotPassword
	baseUrl, err := a.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = util.ReadJSON(r, &forgotPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := forgotPassword.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	err = userService.GeneratePasswordResetToken(r.Context(), baseUrl, &forgotPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Password reset token has been sent successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	userService := createUserService(a)

	err := userService.VerifyEmail(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Email has been verified successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var resetPassword models.ResetPassword
	err := util.ReadJSON(r, &resetPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := resetPassword.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	user, err := userService.ResetPassword(r.Context(), token, &resetPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
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
