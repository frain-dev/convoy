package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
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
		OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
		OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    postgres.NewConfigRepo(h.A.DB),
		LicenseKey:    configuration.LicenseKey,
		Host:          configuration.Host,
		Licenser:      h.A.Licenser,
	}

	resp, err := lu.Run()
	if err != nil {
		h.A.Logger.WithError(err).Errorf("SSO initialization failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
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
		OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
		OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    postgres.NewConfigRepo(h.A.DB),
		LicenseKey:    configuration.LicenseKey,
		Licenser:      h.A.Licenser,
	}

	tokenResp, err := lu.RedeemToken(r.URL.Query())
	if err != nil {
		h.A.Logger.WithError(err).Errorf("SSO token redemption failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
		return
	}

	var user *datastore.User
	var token *jwt.Token
	if intent == RegisterIntent {
		user, token, err = lu.RegisterSSOUser(r.Context(), h.A, tokenResp)
		if err != nil {
			if errors.Is(err, services.ErrUserAlreadyExist) {
				h.A.Logger.WithError(err).Errorf("SSO user registration failed - user already exists: %v", err)
				_ = render.Render(w, r, util.NewErrorResponse("User already exists", http.StatusConflict))
				return
			}

			h.A.Logger.WithError(err).Errorf("SSO user registration failed: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Registration failed", http.StatusForbidden))
			return
		}
	} else {
		user, token, err = lu.LoginSSOUser(r.Context(), tokenResp)
		if err != nil {
			if errors.Is(err, datastore.ErrUserNotFound) {
				h.A.Logger.WithError(err).Errorf("SSO login failed - user not found: %v", err)
				_ = render.Render(w, r, util.NewErrorResponse("Invalid credentials", http.StatusNotFound))
				return
			}
			h.A.Logger.WithError(err).Errorf("SSO login failed: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
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
		h.A.Logger.WithError(err).Errorf("Failed to parse login request body: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		h.A.Logger.Errorf("Failed to get configuration: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Service temporarily unavailable", http.StatusInternalServerError))
		return
	}

	lu := services.LoginUserService{
		UserRepo:      postgres.NewUserRepo(h.A.DB),
		OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
		Cache:         h.A.Cache,
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		Data:          &newUser,
		Licenser:      h.A.Licenser,
	}

	user, token, err := lu.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Errorf("User login failed: %v", err)

		var errMsg string

		if se, ok := err.(*services.ServiceError); ok {
			switch se.Code {
			case services.ErrCodeLicenseExpired:
				errMsg = se.ErrMsg
			default:
				errMsg = "Invalid credentials"
			}
		}

		_ = render.Render(w, r, util.NewErrorResponse(errMsg, http.StatusForbidden))

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
		h.A.Logger.WithError(err).Errorf("Failed to parse refresh token request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := refreshToken.Validate(); err != nil {
		h.A.Logger.WithError(err).Errorf("Refresh token validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid token", http.StatusBadRequest))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		h.A.Logger.Errorf("Failed to get configuration: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Service temporarily unavailable", http.StatusInternalServerError))
		return
	}

	rf := services.RefreshTokenService{
		UserRepo: postgres.NewUserRepo(h.A.DB),
		JWT:      jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		Data:     &refreshToken,
	}

	token, err := rf.Run(r.Context())
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Token refresh failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid or expired token", http.StatusUnauthorized))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Token refresh successful", token, http.StatusOK))
}

func (h *Handler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	auth, err := middleware.GetAuthFromRequest(r)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Failed to get auth from request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Authentication required", http.StatusUnauthorized))
		return
	}

	configuration, err := config.Get()
	if err != nil {
		h.A.Logger.Errorf("Failed to get configuration: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Service temporarily unavailable", http.StatusInternalServerError))
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

// GoogleOAuthToken handles Google ID token authentication
func (h *Handler) GoogleOAuthToken(w http.ResponseWriter, r *http.Request) {
	configuration := h.A.Cfg

	if !configuration.Auth.GoogleOAuth.Enabled {
		_ = render.Render(w, r, util.NewErrorResponse("Google OAuth is not enabled", http.StatusForbidden))
		return
	}

	var request struct {
		IDToken string `json:"id_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("invalid request body", http.StatusBadRequest))
		return
	}

	if request.IDToken == "" {
		_ = render.Render(w, r, util.NewErrorResponse("missing ID token", http.StatusBadRequest))
		return
	}

	googleOAuthService := services.NewGoogleOAuthService(
		postgres.NewUserRepo(h.A.DB),
		organisations.New(h.A.Logger, h.A.DB),
		organisation_members.New(h.A.Logger, h.A.DB),
		jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		postgres.NewConfigRepo(h.A.DB),
		h.A.Licenser,
	)

	user, token, err := googleOAuthService.HandleIDToken(r.Context(), request.IDToken, h.A)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("Google OAuth authentication failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
		return
	}

	// User exists but has no organization - redirect to setup
	if token == nil {
		u := &models.LoginUserResponse{
			User:       user,
			Token:      models.Token{},
			NeedsSetup: true,
		}
		_ = render.Render(w, r, util.NewServerResponse("Google OAuth login successful", u, http.StatusOK))
		return
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Google OAuth login successful", u, http.StatusOK))
}

// GoogleOAuthSetup completes the user setup process for new Google OAuth users
func (h *Handler) GoogleOAuthSetup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		BusinessName string `json:"business_name"`
		IDToken      string `json:"id_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if request.BusinessName == "" {
		_ = render.Render(w, r, util.NewErrorResponse("Business name is required", http.StatusBadRequest))
		return
	}

	if request.IDToken == "" {
		_ = render.Render(w, r, util.NewErrorResponse("ID token is required", http.StatusBadRequest))
		return
	}

	configuration := h.A.Cfg
	googleOAuthService := services.NewGoogleOAuthService(
		postgres.NewUserRepo(h.A.DB),
		organisations.New(h.A.Logger, h.A.DB),
		organisation_members.New(h.A.Logger, h.A.DB),
		jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		postgres.NewConfigRepo(h.A.DB),
		h.A.Licenser,
	)

	user, token, err := googleOAuthService.CompleteGoogleOAuthSetup(r.Context(), request.IDToken, request.BusinessName, h.A)
	if err != nil {
		h.A.Logger.Errorf("Google OAuth setup failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Failed to complete setup", http.StatusInternalServerError))
		return
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}

	_ = render.Render(w, r, util.NewServerResponse("Setup completed successfully", u, http.StatusOK))
}
