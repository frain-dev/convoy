package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

// LoginUser
// @Summary Login a user
// @Description This endpoint logs in a user
// @Tags User
// @Accept  json
// @Produce  json
// @Param user body models.LoginUser true "User Details"
// @Success 200 {object} serverResponse{data=models.LoginUserResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Router /auth/login [post]
func (s *Server) LoginUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.LoginUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, token, err := s.userService.LoginUser(r.Context(), &newUser)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	u := &models.LoginUserResponse{
		UID:       user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Role:      user.Role,
		Token:     models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		DeletedAt: user.DeletedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Login successful", u, http.StatusOK))
}

// RefreshToken
// @Summary Refresh an access token
// @Description This endpoint refreshes an access token
// @Tags User
// @Accept  json
// @Produce  json
// @Param token body models.Token true "Token Details"
// @Success 200 {object} serverResponse{data=models.Token}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Router /auth/token/refresh [post]
func (s *Server) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var refreshToken models.Token
	if err := util.ReadJSON(r, &refreshToken); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	token, err := s.userService.RefreshToken(r.Context(), &refreshToken)
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
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /auth/logout [post]
func (s *Server) LogoutUser(w http.ResponseWriter, r *http.Request) {
	auth, err := m.GetAuthFromRequest(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	err = s.userService.LogoutUser(auth.Token)
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
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /users/{userID}/profile [get]
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
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
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /users/{userID}/profile [put]
func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
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

	user, err = s.userService.UpdateUser(r.Context(), &userUpdate, user)
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
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /users/{userID}/password [put]
func (s *Server) UpdatePassword(w http.ResponseWriter, r *http.Request) {
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

	user, err = s.userService.UpdatePassword(r.Context(), &updatePassword, user)
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
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Router /users/forgot-password [post]
func (s *Server) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var forgotPassword models.ForgotPassword
	baseUrl := m.GetHostFromContext(r.Context())

	err := util.ReadJSON(r, &forgotPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = s.userService.GeneratePasswordResetToken(r.Context(), baseUrl, &forgotPassword)
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
// @Success 200 {object} serverResponse{data=datastore.User}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Router /users/reset-password [post]
func (s *Server) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var resetPassword models.ResetPassword
	err := util.ReadJSON(r, &resetPassword)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, err := s.userService.ResetPassword(r.Context(), token, &resetPassword)
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
