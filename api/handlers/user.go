package handlers

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

const (
	ErrDisposableEmailNotAllowed = "disposable email not allowed."
)

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.RegisterUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if newUser.EmailIsDisposable() {
		_ = render.Render(w, r, util.NewErrorResponse(ErrDisposableEmailNotAllowed, http.StatusBadRequest))
		return
	}

	if err := newUser.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	config, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	rs := services.RegisterUserService{
		UserRepo:      postgres.NewUserRepo(h.A.DB, h.A.Cache),
		OrgRepo:       postgres.NewOrgRepo(h.A.DB, h.A.Cache),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB, h.A.Cache),
		Queue:         h.A.Queue,
		JWT:           jwt.NewJwt(&config.Auth.Jwt, h.A.Cache),
		ConfigRepo:    postgres.NewConfigRepo(h.A.DB),
		BaseURL:       baseUrl,
		Data:          &newUser,
	}

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
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Queue:    h.A.Queue,
		BaseURL:  baseUrl,
		User:     user,
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

	u := services.UpdateUserService{
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Data:     &userUpdate,
		User:     user,
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

	up := services.UpdatePasswordService{
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Data:     &updatePassword,
		User:     user,
	}

	user, err = up.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
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
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := forgotPassword.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	gp := services.GeneratePasswordResetTokenService{
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Queue:    h.A.Queue,
		BaseURL:  baseUrl,
		Data:     &forgotPassword,
	}

	err = gp.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Password reset token has been sent successfully", nil, http.StatusOK))
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ve := services.VerifyEmailService{
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Token:    r.URL.Query().Get("token"),
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
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := resetPassword.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	rs := services.ResetPasswordService{
		UserRepo: postgres.NewUserRepo(h.A.DB, h.A.Cache),
		Token:    token,
		Data:     &resetPassword,
	}

	user, err := rs.Run(r.Context())
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
