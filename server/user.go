package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
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
func (a *applicationHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.LoginUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, token, err := a.userService.LoginUser(r.Context(), &newUser)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	_ = render.Render(w, r, newServerResponse("Login successful", u, http.StatusOK))
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
func (a *applicationHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var refreshToken models.Token
	if err := util.ReadJSON(r, &refreshToken); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	token, err := a.userService.RefreshToken(r.Context(), &refreshToken)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Token refresh successful", token, http.StatusOK))
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
func (a *applicationHandler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	auth, err := getAuthFromRequest(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	err = a.userService.LogoutUser(auth.Token)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Logout successful", nil, http.StatusOK))
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
func (a *applicationHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	_ = render.Render(w, r, newServerResponse("User fetched successfully", user, http.StatusOK))
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
func (a *applicationHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var userUpdate models.UpdateUser
	err := util.ReadJSON(r, &userUpdate)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	user, err = a.userService.UpdateUser(r.Context(), &userUpdate, user)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("User updated successfully", user, http.StatusOK))
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
func (a *applicationHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var updatePassword models.UpdatePassword
	err := util.ReadJSON(r, &updatePassword)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user, ok := getUser(r)
	if !ok {
		_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	user, err = a.userService.UpdatePassword(r.Context(), &updatePassword, user)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse("Password updated successfully", user, http.StatusOK))

}

func getUser(r *http.Request) (*datastore.User, bool) {
	authUser := getAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)

	return user, ok
}
