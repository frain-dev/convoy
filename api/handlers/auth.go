package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/pkg/sso/service"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) InitSSO(w http.ResponseWriter, r *http.Request) {
	configuration := h.A.Cfg
	billingEnabled := configuration.Billing.Enabled && h.A.BillingClient != nil
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))

	licenseKey := configuration.LicenseKey
	if billingEnabled && slug != "" {
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		orgMemberRepo := organisation_members.New(h.A.Logger, h.A.DB)
		result, err := services.ResolveWorkspaceBySlug(r.Context(), slug, services.ResolveWorkspaceBySlugDeps{
			BillingClient: h.A.BillingClient,
			OrgRepo:       orgRepo,
			Logger:        h.A.Logger,
			Cfg:           configuration,
			RefreshDeps: services.RefreshLicenseDataDeps{
				OrgMemberRepo: orgMemberRepo,
				OrgRepo:       orgRepo,
				BillingClient: h.A.BillingClient,
				Logger:        h.A.Logger,
				Cfg:           configuration,
			},
		})
		if err != nil {
			h.A.Logger.WithError(err).WithField("slug", slug).Debug("InitSSO: workspace resolve failed")
			_ = render.Render(w, r, util.NewErrorResponse("Workspace not found", http.StatusBadRequest))
			return
		}
		if !result.SSOAvailable {
			_ = render.Render(w, r, util.NewErrorResponse("SSO is not available for this workspace", http.StatusBadRequest))
			return
		}
		licenseKey = result.LicenseKey
	}

	lu := services.LoginUserSSOService{
		UserRepo:      users.New(h.A.Logger, h.A.DB),
		OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
		OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    h.A.ConfigRepo,
		LicenseKey:    licenseKey,
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

func (h *Handler) RedeemSSOCallback(w http.ResponseWriter, r *http.Request) {
	configuration := h.A.Cfg
	lu := services.LoginUserSSOService{
		UserRepo:      users.New(h.A.Logger, h.A.DB),
		OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
		OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
		JWT:           jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		ConfigRepo:    h.A.ConfigRepo,
		LicenseKey:    configuration.LicenseKey,
		Licenser:      h.A.Licenser,
	}

	tokenResp, err := lu.RedeemToken(r.URL.Query())
	if err != nil {
		h.A.Logger.WithError(err).Errorf("SSO token redemption failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
		return
	}

	user, token, err := lu.LoginSSOUser(r.Context(), tokenResp)
	if err != nil {
		if !errors.Is(err, datastore.ErrUserNotFound) {
			h.A.Logger.WithError(err).Errorf("SSO callback login failed: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Authentication failed", http.StatusForbidden))
			return
		}
		user, token, err = lu.RegisterSSOUser(r.Context(), h.A, tokenResp)
		if err != nil && errors.Is(err, services.ErrUserAlreadyExist) {
			user, token, err = lu.LoginSSOUser(r.Context(), tokenResp)
		}
		if err != nil {
			h.A.Logger.WithError(err).Errorf("SSO callback registration failed: %v", err)
			_ = render.Render(w, r, util.NewErrorResponse("Registration failed", http.StatusForbidden))
			return
		}
	}

	if configuration.Billing.Enabled {
		go services.RefreshLicenseDataForUser(user.UID, services.RefreshLicenseDataDeps{
			OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
			OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
			BillingClient: h.A.BillingClient,
			Logger:        h.A.Logger,
			Cfg:           h.A.Cfg,
		})
	}

	u := &models.LoginUserResponse{
		User:  user,
		Token: models.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken},
	}
	_ = render.Render(w, r, util.NewServerResponse("Login successful", u, http.StatusOK))
}

type adminPortalRequest struct {
	ReturnURL  string `json:"return_url"`
	SuccessURL string `json:"success_url"`
}

func (h *Handler) GetSSOAdminPortal(w http.ResponseWriter, r *http.Request) {
	configuration := h.A.Cfg
	if configuration.LicenseKey == "" {
		h.A.Logger.Error("SSO admin portal: missing license key")
		_ = render.Render(w, r, util.NewErrorResponse("SSO not configured", http.StatusForbidden))
		return
	}

	var body adminPortalRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	returnURL := strings.TrimSpace(body.ReturnURL)
	successURL := strings.TrimSpace(body.SuccessURL)
	if returnURL == "" {
		returnURL = configuration.Host
		if returnURL != "" && !strings.HasPrefix(returnURL, "http://") && !strings.HasPrefix(returnURL, "https://") {
			returnURL = "https://" + returnURL
		}
	}
	if returnURL == "" {
		_ = render.Render(w, r, util.NewErrorResponse("return_url is required", http.StatusBadRequest))
		return
	}

	sc := service.Config{
		Host:            configuration.SSOService.Host,
		RedirectPath:    configuration.SSOService.RedirectPath,
		TokenPath:       configuration.SSOService.TokenPath,
		AdminPortalPath: configuration.SSOService.AdminPortalPath,
		Timeout:         configuration.SSOService.Timeout,
		RetryCount:      configuration.SSOService.RetryCount,
	}
	if configuration.Billing.Enabled && configuration.Billing.APIKey != "" {
		sc.APIKey = configuration.Billing.APIKey
		sc.LicenseKey = configuration.LicenseKey
		sc.OrgID = r.Header.Get("X-Organisation-Id")
	}
	ssoClient := service.NewClient(sc)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := ssoClient.GetAdminPortalURL(ctx, configuration.LicenseKey, returnURL, successURL)
	if err != nil {
		h.A.Logger.WithError(err).Errorf("SSO admin portal failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Failed to get SSO admin portal URL", http.StatusForbidden))
		return
	}

	data := map[string]interface{}{
		"portal_url": resp.Data.PortalURL,
		"expires_in": resp.Data.ExpiresIn,
	}
	_ = render.Render(w, r, util.NewServerResponse("Admin portal URL generated", data, http.StatusOK))
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
		UserRepo:      users.New(h.A.Logger, h.A.DB),
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

	if configuration.Billing.Enabled {
		go services.RefreshLicenseDataForUser(user.UID, services.RefreshLicenseDataDeps{
			OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
			OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
			BillingClient: h.A.BillingClient,
			Logger:        h.A.Logger,
			Cfg:           h.A.Cfg,
		})
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
		UserRepo: users.New(h.A.Logger, h.A.DB),
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
		UserRepo: users.New(h.A.Logger, h.A.DB),
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
		users.New(h.A.Logger, h.A.DB),
		organisations.New(h.A.Logger, h.A.DB),
		organisation_members.New(h.A.Logger, h.A.DB),
		jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		h.A.ConfigRepo,
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
		users.New(h.A.Logger, h.A.DB),
		organisations.New(h.A.Logger, h.A.DB),
		organisation_members.New(h.A.Logger, h.A.DB),
		jwt.NewJwt(&configuration.Auth.Jwt, h.A.Cache),
		h.A.ConfigRepo,
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
