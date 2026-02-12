package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/sso/service"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type LoginUserSSOService struct {
	UserRepo      datastore.UserRepository
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	JWT           *jwt.Jwt
	ConfigRepo    datastore.ConfigurationRepository
	Licenser      license.Licenser
	SSOClient     *service.Client

	LicenseKey string
	Host       string
}

func (u *LoginUserSSOService) Run() (*models.SSOLoginResponse, error) {
	if u.LicenseKey == "" {
		return nil, errors.New("missing license key")
	}

	cfg, err := config.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	ssoClient := u.SSOClient
	if ssoClient == nil {
		sc := service.Config{
			Host:            cfg.SSOService.Host,
			RedirectPath:    cfg.SSOService.RedirectPath,
			TokenPath:       cfg.SSOService.TokenPath,
			AdminPortalPath: cfg.SSOService.AdminPortalPath,
			Timeout:         cfg.SSOService.Timeout,
			RetryCount:      cfg.SSOService.RetryCount,
		}
		if cfg.Billing.Enabled && cfg.Billing.APIKey != "" {
			sc.APIKey = cfg.Billing.APIKey
			sc.LicenseKey = u.LicenseKey
		}
		ssoClient = service.NewClient(sc)
	}

	redirectURI := strings.TrimSpace(cfg.Auth.SSO.RedirectURL)
	if redirectURI == "" {
		return nil, errors.New("auth.sso.redirect_url is required for SSO login")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	redirectResp, err := ssoClient.GetRedirectURL(ctx, u.LicenseKey, u.Host, redirectURI)
	if err != nil {
		log.Errorf("failed to get SSO redirect URL: %+v", err)
		return nil, fmt.Errorf("failed to get SSO redirect URL: %w", err)
	}

	if redirectResp.Data.RedirectURL == "" {
		return nil, errors.New("no redirect URL in SSO response")
	}

	log.Infof("SSO redirect URL obtained successfully")

	return &models.SSOLoginResponse{
		RedirectURL: redirectResp.Data.RedirectURL,
	}, nil
}

func (u *LoginUserSSOService) RedeemToken(queryValues url.Values) (*models.SSOTokenResponse, error) {
	token := queryValues.Get("token")
	if token == "" {
		token = queryValues.Get("sso_token")
	}
	if token == "" {
		return nil, errors.New("missing token")
	}

	cfg, err := config.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	ssoClient := u.SSOClient
	if ssoClient == nil {
		sc := service.Config{
			Host:            cfg.SSOService.Host,
			RedirectPath:    cfg.SSOService.RedirectPath,
			TokenPath:       cfg.SSOService.TokenPath,
			AdminPortalPath: cfg.SSOService.AdminPortalPath,
			Timeout:         cfg.SSOService.Timeout,
			RetryCount:      cfg.SSOService.RetryCount,
		}
		if cfg.Billing.Enabled && cfg.Billing.APIKey != "" {
			sc.APIKey = cfg.Billing.APIKey
			sc.LicenseKey = u.LicenseKey
		}
		ssoClient = service.NewClient(sc)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tokenResp, err := ssoClient.ValidateToken(ctx, token)
	if err != nil {
		log.Errorf("failed to validate SSO token: %+v", err)
		return nil, fmt.Errorf("failed to validate SSO token: %w", err)
	}

	payload := models.Payload{
		Email:                  tokenResp.Data.Payload.Email,
		FirstName:              tokenResp.Data.Payload.FirstName,
		LastName:               tokenResp.Data.Payload.LastName,
		OrganizationID:         tokenResp.Data.Payload.OrganizationID,
		OrganizationExternalID: tokenResp.Data.Payload.OrganizationExternalID,
		ID:                     tokenResp.Data.Payload.ID,
	}

	if payload.Email == "" {
		return nil, errors.New("no email in token response")
	}
	if payload.OrganizationExternalID == "" {
		return nil, errors.New("no external organization id in token response")
	}

	log.Infof("SSO token validated successfully")

	return &models.SSOTokenResponse{
		Status:  true,
		Message: "Token validated successfully",
		Data: struct {
			Payload models.Payload `json:"payload"`
		}{
			Payload: payload,
		},
	}, nil
}

func (u *LoginUserSSOService) LoginSSOUser(ctx context.Context, t *models.SSOTokenResponse) (*datastore.User, *jwt.Token, error) {
	user, err := u.UserRepo.FindUserByEmail(ctx, t.Data.Payload.Email)
	if user != nil && err == nil {
		token, err := u.JWT.GenerateToken(user)
		if err != nil {
			return nil, nil, &ServiceError{ErrMsg: err.Error()}
		}

		return user, &token, nil
	}

	if errors.Is(err, datastore.ErrUserNotFound) {
		return nil, nil, err
	}

	return nil, nil, &ServiceError{ErrMsg: "login failed", Err: err}
}

func (u *LoginUserSSOService) RegisterSSOUser(ctx context.Context, a *types.APIOptions, t *models.SSOTokenResponse) (*datastore.User, *jwt.Token, error) {
	user, err := u.UserRepo.FindUserByEmail(ctx, t.Data.Payload.Email)
	if user != nil && err == nil {
		return nil, nil, &ServiceError{
			ErrMsg: ErrUserAlreadyExist.Error(),
			Err:    ErrUserAlreadyExist,
		}
	}

	ok, err := a.Licenser.CheckUserLimit(ctx)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}
	if !ok {
		return nil, nil, &ServiceError{ErrMsg: ErrUserLimit.Error()}
	}

	cfg, err := u.ConfigRepo.LoadConfiguration(ctx)
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		return nil, nil, &ServiceError{ErrMsg: "failed to load configuration", Err: err}
	}
	if cfg != nil {
		if !cfg.IsSignupEnabled {
			// registration is not allowed
			return nil, nil, &ServiceError{ErrMsg: datastore.ErrSignupDisabled.Error(), Err: datastore.ErrSignupDisabled}
		}
	}

	p := datastore.Password{Plaintext: ulid.Make().String()}
	err = p.GenerateHash()

	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate hash")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate hash", Err: err}
	}

	firstName := t.Data.Payload.FirstName
	lastName := t.Data.Payload.LastName
	if firstName == "" || lastName == "" {
		firstName, lastName = util.ExtractOrGenerateNamesFromEmail(t.Data.Payload.Email)
	}
	user = &datastore.User{
		UID:                    ulid.Make().String(),
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  t.Data.Payload.Email,
		Password:               string(p.Hash),
		EmailVerificationToken: ulid.Make().String(),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
		EmailVerified:          true,
		AuthType:               string(datastore.SSOUserType),
	}

	err = u.UserRepo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			return nil, nil, &ServiceError{ErrMsg: "this email is taken"}
		}

		log.FromContext(ctx).WithError(err).Error("failed to create user")
		return nil, nil, &ServiceError{ErrMsg: "failed to create user", Err: err}
	}

	co := CreateOrganisationService{
		OrgRepo:       u.OrgRepo,
		OrgMemberRepo: u.OrgMemberRepo,
		Licenser:      u.Licenser,
		Logger:        log.FromContext(ctx),
		NewOrg:        &datastore.OrganisationRequest{Name: t.Data.Payload.OrganizationExternalID},
		User:          user,
	}

	_, err = co.Run(ctx)
	if err != nil {
		if !errors.Is(err, ErrOrgLimit) && !errors.Is(err, ErrUserLimit) {
			return nil, nil, err
		}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate token")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate token", Err: err}
	}

	return user, &token, nil
}
