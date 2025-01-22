package handlers

import (
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"net/http"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

type (
	SSOAuthIntent string
)

const (
	LoginIntent    SSOAuthIntent = "login"
	RegisterIntent SSOAuthIntent = "register"
)

func (h *Handler) InitSSO(w http.ResponseWriter, r *http.Request) {

	configuration := h.A.Cfg

	lu := services.LoginUserSSOService{
		UserRepo:      postgres.NewUserRepo(h.A.DB),
		OrgRepo:       postgres.NewOrgRepo(h.A.DB),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    postgres.NewConfigRepo(h.A.DB),
		LicenseKey:    configuration.LicenseKey,
		Host:          configuration.Host,
		Licenser:      h.A.Licenser,
	}

	resp, err := lu.Run()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusForbidden))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Get Redirect successful", resp, http.StatusOK))
}

func (h *Handler) RedeemLoginSSOToken(w http.ResponseWriter, r *http.Request) {
	h.redeemSSOToken(w, r, LoginIntent)
}

func (h *Handler) RedeemRegisterSSOToken(w http.ResponseWriter, r *http.Request) {
	h.redeemSSOToken(w, r, RegisterIntent)
}

func (h *Handler) redeemSSOToken(w http.ResponseWriter, r *http.Request, intent SSOAuthIntent) {

	configuration := h.A.Cfg

	lu := services.LoginUserSSOService{
		UserRepo:      postgres.NewUserRepo(h.A.DB),
		OrgRepo:       postgres.NewOrgRepo(h.A.DB),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    postgres.NewConfigRepo(h.A.DB),
		LicenseKey:    configuration.LicenseKey,
		Licenser:      h.A.Licenser,
	}

	tokenResp, err := lu.RedeemToken(r.URL.Query())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusForbidden))
		return
	}

	var user *datastore.User
	var token *jwt.Token
	if intent == RegisterIntent {
		user, token, err = lu.RegisterSSOUser(r.Context(), h.A, tokenResp)
		if err != nil {
			if errors.Is(err, services.ErrUserAlreadyExist) {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusConflict))
				return
			}
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusForbidden))
			return
		}

	} else {
		user, token, err = lu.LoginSSOUser(r.Context(), tokenResp)
		if err != nil {
			if errors.Is(err, datastore.ErrUserNotFound) {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
				return
			}
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusForbidden))
			return
		}
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Login successful", u, http.StatusOK))
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.LoginUser
	if err := util.ReadJSON(r, &newUser); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	lu := services.LoginUserService{
		UserRepo: postgres.NewUserRepo(h.A.DB),
		Cache:    h.A.Cache,
		JWT:      jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		Data:     &newUser,
	}

	user, token, err := lu.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusForbidden))
		return
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Login successful", u, http.StatusOK))
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var refreshToken models.Token
	if err := util.ReadJSON(r, &refreshToken); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := refreshToken.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	rf := services.RefreshTokenService{
		UserRepo: postgres.NewUserRepo(h.A.DB),
		JWT:      jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		Data:     &refreshToken,
	}

	token, err := rf.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Token refresh successful", token, http.StatusOK))
}

func (h *Handler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	auth, err := middleware.GetAuthFromRequest(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	lg := services.LogoutUserService{
		UserRepo: postgres.NewUserRepo(h.A.DB),
		JWT:      jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		Token:    auth.Token,
	}

	err = lg.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Logout successful", nil, http.StatusOK))
}
