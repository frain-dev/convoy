package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createUserService(a *ApplicationHandler) *services.UserService {
	userRepo := mongo.NewUserRepo(a.A.Store)
	configService := createConfigService(a)
	orgService := createOrganisationService(a)

	return services.NewUserService(
		userRepo, a.A.Cache, a.A.Queue,
		configService, orgService,
	)
}

// LoginUser
// @Summary Login a user
// @Description This endpoint logs in a user
// @Tags User
// @Accept  json
// @Produce  json
// @Param user body models.LoginUser true "User Details"
// @Success 200 {object} util.ServerResponse{data=models.LoginUserResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /ui/auth/login [post]
func (a *ApplicationHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.LoginUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	user, token, err := userService.LoginUser(r.Context(), &newUser)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := &models.LoginUserResponse{
		UID:       user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Token:     models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Login successful", u, http.StatusOK))
}

// RegisterUser
// @Summary Registers a user
// @Description This endpoint registers a new user
// @Tags User
// @Accept  json
// @Produce  json
// @Param user body models.RegisterUser true "User Details"
// @Success 201 {object} util.ServerResponse{data=models.LoginUserResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /ui/auth/register [post]
func (a *ApplicationHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.RegisterUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	user, token, err := userService.RegisterUser(r.Context(), &newUser)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := &models.LoginUserResponse{
		UID:       user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Token:     models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Registration successful", u, http.StatusCreated))
}

// RefreshToken
// @Summary Refresh an access token
// @Description This endpoint refreshes an access token
// @Tags User
// @Accept  json
// @Produce  json
// @Param token body models.Token true "Token Details"
// @Success 200 {object} util.ServerResponse{data=models.Token}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /ui/auth/token/refresh [post]
func (a *ApplicationHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var refreshToken models.Token
	if err := util.ReadJSON(r, &refreshToken); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	token, err := userService.RefreshToken(r.Context(), &refreshToken)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Token refresh successful", token, http.StatusOK))
}

// LogoutUser
// @Summary Logs out a user
// @Description This endpoint logs out a user
// @Tags User
// @Accept  json
// @Produce  json
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/auth/logout [post]
func (a *ApplicationHandler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	auth, err := m.GetAuthFromRequest(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	userService := createUserService(a)
	err = userService.LogoutUser(auth.Token)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Logout successful", nil, http.StatusOK))
}

// GetUser
// @Summary Gets a user
// @Description This endpoint fetches a user
// @Tags User
// @Accept  json
// @Produce  json
// @Param userID path string true "user id"
// @Success 200 {object} util.ServerResponse{data=datastore.User}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/profile [get]
func (a *ApplicationHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("User fetched successfully", user, http.StatusOK))
}

// UpdateUser
// @Summary Updates a user
// @Description This endpoint updates a user
// @Tags User
// @Accept  json
// @Produce  json
// @Param userID path string true "user id"
// @Param group body models.UpdateUser true "User Details"
// @Success 200 {object} util.ServerResponse{data=datastore.User}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/profile [put]
func (a *ApplicationHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var userUpdate models.UpdateUser
	err := util.ReadJSON(r, &userUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	userService := createUserService(a)
	user, err = userService.UpdateUser(r.Context(), &userUpdate, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("User updated successfully", user, http.StatusOK))
}

// UpdatePassword
// @Summary Updates a user's password
// @Description This endpoint updates a user's password
// @Tags User
// @Accept  json
// @Produce  json
// @Param userID path string true "user id"
// @Param group body models.UpdatePassword true "Password Details"
// @Success 200 {object} util.ServerResponse{data=datastore.User}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/password [put]
func (a *ApplicationHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var updatePassword models.UpdatePassword
	err := util.ReadJSON(r, &updatePassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	userService := createUserService(a)
	user, err = userService.UpdatePassword(r.Context(), &updatePassword, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Password updated successfully", user, http.StatusOK))
}

// ForgotPassword
// @Summary Send password reset token
// @Description This endpoint generates a password reset token
// @Tags User
// @Accept  json
// @Produce  json
// @Param email body models.ForgotPassword true "Forgot Password Details"
// @Success 200 {object} util.ServerResponse{data=datastore.User}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /ui/users/forgot-password [post]
func (a *ApplicationHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var forgotPassword models.ForgotPassword
	baseUrl := m.GetHostFromContext(r.Context())

	err := util.ReadJSON(r, &forgotPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	err = userService.GeneratePasswordResetToken(r.Context(), baseUrl, &forgotPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Password reset token has been sent succesfully", nil, http.StatusOK))
}

// ResetPassword
// @Summary Reset user password
// @Description This endpoint resets a users password
// @Tags User
// @Accept  json
// @Produce  json
// @Param token query string true "reset token"
// @Param password body models.ResetPassword true "Reset Password Details"
// @Success 200 {object} util.ServerResponse{data=datastore.User}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Router /ui/users/reset-password [post]
func (a *ApplicationHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var resetPassword models.ResetPassword
	err := util.ReadJSON(r, &resetPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	userService := createUserService(a)
	user, err := userService.ResetPassword(r.Context(), token, &resetPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("password reset succesful", user, http.StatusOK))
}

func getUser(r *http.Request) (*datastore.User, bool) {
	authUser := m.GetAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)

	return user, ok
}
