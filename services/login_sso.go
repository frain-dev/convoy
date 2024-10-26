package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"io"
	"net/http"
	"net/url"
	"time"
)

type LoginUserSSOService struct {
	UserRepo      datastore.UserRepository
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	JWT           *jwt.Jwt
	ConfigRepo    datastore.ConfigurationRepository
	Licenser      license.Licenser

	LicenseKey string
}

const ssoUrl = "https://ssoproxy.getconvoy.io"
const ssoRedirectPath = "/ssoready/redirect"
const ssoTokenPath = "/ssoready/token"

func (u *LoginUserSSOService) Run() (*models.SSOLoginResponse, error) {

	ssoReq := models.SSORequest{
		LicenseKey: u.LicenseKey,
	}

	if ssoReq.LicenseKey == "" {
		return nil, errors.New("missing license key")
	}

	b, err := json.Marshal(ssoReq)
	if err != nil {
		return nil, errors.New("failed to marshal payload")
	}

	rUrl := ssoUrl + ssoRedirectPath
	req, err := http.NewRequest("POST", rUrl, bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("failed to create SSO request")
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to call SSO: %+v", err)
		return nil, errors.New("failed to call SSO")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("failed to read SSO response")
	}

	var ssoResp models.SSOResponse
	if err := json.Unmarshal(body, &ssoResp); err != nil {
		return nil, errors.New("failed to parse SSO JSON response")
	}

	if !ssoResp.Status || ssoResp.Data.RedirectURL == "" {
		log.Errorf("no valid redirectUrl in SSO response: %+v", ssoResp)
		return nil, errors.New("no redirect URL in SSO response")
	}

	log.Infof("Login should be successful: %+v", ssoResp)

	return &models.SSOLoginResponse{
		RedirectURL: ssoResp.Data.RedirectURL,
	}, nil
}

func (u *LoginUserSSOService) RedeemToken(queryValues url.Values) (*models.SSOTokenResponse, error) {
	accessCode := queryValues.Get("saml_access_code")

	if accessCode == "" {
		return nil, errors.New("missing saml_access_code")
	}

	tokenRequest := models.SSOTokenRequest{
		Token: accessCode,
	}

	b, err := json.Marshal(tokenRequest)
	if err != nil {
		return nil, errors.New("failed to marshal payload")
	}

	rUrl := ssoUrl + ssoTokenPath
	req, err := http.NewRequest("POST", rUrl, bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("failed to create SSO token request")
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to call SSO: %+v", err)
		return nil, errors.New("failed to call SSO for token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("failed to read SSO token response")
	}

	var tokenResponse models.SSOTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, errors.New("failed to parse SSO token JSON response")
	}

	if !tokenResponse.Status {
		log.Errorf("failed to redeem token in response: %+v", tokenResponse)
		return nil, errors.New("failed to redeem token")
	}

	if tokenResponse.Data.Payload.Email == "" {
		return nil, errors.New("no email in token response")
	}
	if tokenResponse.Data.Payload.OrganizationExternalID == "" {
		return nil, errors.New("no external organization id in token response")
	}

	log.Infof("Token should be successful: %+v", tokenResponse)

	return &tokenResponse, nil
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
		return nil, nil, &ServiceError{ErrMsg: err.Error(), Err: err}
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

	ok, err := a.Licenser.CreateUser(ctx)
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

	firstName, lastName := util.ExtractOrGenerateNamesFromEmail(t.Data.Payload.Email)
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
		NewOrg:        &models.Organisation{Name: t.Data.Payload.OrganizationExternalID},
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
