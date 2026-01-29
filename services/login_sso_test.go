package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/sso/service"
	"github.com/frain-dev/convoy/mocks"
)

func provideLoginUserSSOService(ctrl *gomock.Controller, t *testing.T) (*LoginUserSSOService, *httptest.Server) {
	cfg, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	jwtInstance := jwt.NewJwt(&cfg.Auth.Jwt, c)

	// Create a mock SSO server
	ssoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sso/redirect":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(service.RedirectURLResponse{
				Status:  true,
				Message: "Success",
				Data:    service.RedirectURLData{RedirectURL: "https://workos.com/authorize?client_id=test"},
			})
		case "/sso/token":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(service.TokenValidationResponse{
				Status:  true,
				Message: "Success",
				Data: service.TokenValidationData{
					Payload: service.UserProfile{
						Email:                  "sso@test.com",
						FirstName:              "SSO",
						LastName:               "User",
						OrganizationID:         "org-123",
						OrganizationExternalID: "test-org",
						ID:                     "user-123",
					},
				},
			})
		}
	}))

	// Create SSO client pointing to mock server
	ssoClient := service.NewClient(service.Config{
		Host:         ssoServer.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	return &LoginUserSSOService{
		UserRepo:      mocks.NewMockUserRepository(ctrl),
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		JWT:           jwtInstance,
		ConfigRepo:    mocks.NewMockConfigurationRepository(ctrl),
		Licenser:      mocks.NewMockLicenser(ctrl),
		SSOClient:     ssoClient,
		LicenseKey:    "test-license-key",
		Host:          "https://convoy.example.com",
	}, ssoServer
}

func TestLoginUserSSOService_Run(t *testing.T) {
	tests := []struct {
		name       string
		licenseKey string
		host       string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "should_get_redirect_url_successfully",
			licenseKey: "test-license-key",
			host:       "https://convoy.example.com",
			wantErr:    false,
		},
		{
			name:       "should_fail_without_license_key",
			licenseKey: "",
			host:       "https://convoy.example.com",
			wantErr:    true,
			wantErrMsg: "missing license key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
			require.NoError(t, err)
			cfg, err := config.Get()
			require.NoError(t, err)
			cfg.Auth.SSO.RedirectURL = "https://convoy.example.com/sso/callback"
			require.NoError(t, config.Override(&cfg))

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service, ssoServer := provideLoginUserSSOService(ctrl, t)
			defer ssoServer.Close()

			service.LicenseKey = tc.licenseKey
			service.Host = tc.host

			resp, err := service.Run()
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotEmpty(t, resp.RedirectURL)
			require.Contains(t, resp.RedirectURL, "workos.com")
		})
	}
}

func TestLoginUserSSOService_RedeemToken(t *testing.T) {
	tests := []struct {
		name       string
		queryVals  url.Values
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_redeem_token_successfully_with_token_param",
			queryVals: url.Values{
				"token": []string{"test-token-123"},
			},
			wantErr: false,
		},
		{
			name: "should_redeem_token_successfully_with_sso_token_param",
			queryVals: url.Values{
				"sso_token": []string{"test-token-123"},
			},
			wantErr: false,
		},
		{
			name:       "should_fail_without_token",
			queryVals:  url.Values{},
			wantErr:    true,
			wantErrMsg: "missing token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
			require.NoError(t, err)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service, ssoServer := provideLoginUserSSOService(ctrl, t)
			defer ssoServer.Close()

			tokenResp, err := service.RedeemToken(tc.queryVals)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tokenResp)
			require.True(t, tokenResp.Status)
			require.Equal(t, "sso@test.com", tokenResp.Data.Payload.Email)
			require.Equal(t, "SSO", tokenResp.Data.Payload.FirstName)
			require.Equal(t, "User", tokenResp.Data.Payload.LastName)
			require.Equal(t, "test-org", tokenResp.Data.Payload.OrganizationExternalID)
		})
	}
}

func TestLoginUserSSOService_LoginSSOUser(t *testing.T) {
	tests := []struct {
		name       string
		tokenResp  *models.SSOTokenResponse
		dbFn       func(u *LoginUserSSOService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_login_existing_user",
			tokenResp: &models.SSOTokenResponse{
				Status:  true,
				Message: "Success",
				Data: struct {
					Payload models.Payload `json:"payload"`
				}{
					Payload: models.Payload{
						Email:                  "existing@test.com",
						FirstName:              "Existing",
						LastName:               "User",
						OrganizationExternalID: "test-org",
					},
				},
			},
			dbFn: func(u *LoginUserSSOService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "existing@test.com").
					Times(1).Return(&datastore.User{
					UID:       "user-123",
					FirstName: "Existing",
					LastName:  "User",
					Email:     "existing@test.com",
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_for_nonexistent_user",
			tokenResp: &models.SSOTokenResponse{
				Status:  true,
				Message: "Success",
				Data: struct {
					Payload models.Payload `json:"payload"`
				}{
					Payload: models.Payload{
						Email:                  "new@test.com",
						FirstName:              "New",
						LastName:               "User",
						OrganizationExternalID: "test-org",
					},
				},
			},
			dbFn: func(u *LoginUserSSOService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "new@test.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantErr:    true,
			wantErrMsg: datastore.ErrUserNotFound.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
			require.NoError(t, err)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service, ssoServer := provideLoginUserSSOService(ctrl, t)
			defer ssoServer.Close()

			if tc.dbFn != nil {
				tc.dbFn(service)
			}

			user, token, err := service.LoginSSOUser(context.Background(), tc.tokenResp)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, user)
			require.NotNil(t, token)
			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)
			require.Equal(t, tc.tokenResp.Data.Payload.Email, user.Email)
		})
	}
}

func TestLoginUserSSOService_RegisterSSOUser(t *testing.T) {
	tests := []struct {
		name       string
		tokenResp  *models.SSOTokenResponse
		dbFn       func(u *LoginUserSSOService, licenser *mocks.MockLicenser)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_register_new_user",
			tokenResp: &models.SSOTokenResponse{
				Status:  true,
				Message: "Success",
				Data: struct {
					Payload models.Payload `json:"payload"`
				}{
					Payload: models.Payload{
						Email:                  "new@test.com",
						FirstName:              "New",
						LastName:               "User",
						OrganizationExternalID: "test-org",
					},
				},
			},
			dbFn: func(u *LoginUserSSOService, licenser *mocks.MockLicenser) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "new@test.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)
				us.EXPECT().CreateUser(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				cfg, _ := u.ConfigRepo.(*mocks.MockConfigurationRepository)
				cfg.EXPECT().LoadConfiguration(gomock.Any()).
					Times(1).Return(&datastore.Configuration{
					IsSignupEnabled: true,
				}, nil)

				licenser.EXPECT().CheckUserLimit(gomock.Any()).
					Times(1).Return(true, nil)

				orgRepo, _ := u.OrgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				orgMemberRepo, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				orgMemberRepo.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				licenser.EXPECT().CheckOrgLimit(gomock.Any()).
					Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).
					Times(1).Return(true, nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_if_user_already_exists",
			tokenResp: &models.SSOTokenResponse{
				Status:  true,
				Message: "Success",
				Data: struct {
					Payload models.Payload `json:"payload"`
				}{
					Payload: models.Payload{
						Email:                  "existing@test.com",
						FirstName:              "Existing",
						LastName:               "User",
						OrganizationExternalID: "test-org",
					},
				},
			},
			dbFn: func(u *LoginUserSSOService, licenser *mocks.MockLicenser) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "existing@test.com").
					Times(1).Return(&datastore.User{
					UID:   "user-123",
					Email: "existing@test.com",
				}, nil)
			},
			wantErr:    true,
			wantErrMsg: ErrUserAlreadyExist.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
			require.NoError(t, err)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service, ssoServer := provideLoginUserSSOService(ctrl, t)
			defer ssoServer.Close()

			licenser := mocks.NewMockLicenser(ctrl)
			service.Licenser = licenser

			apiOpts := &types.APIOptions{
				Licenser: licenser,
			}

			if tc.dbFn != nil {
				tc.dbFn(service, licenser)
			}

			user, token, err := service.RegisterSSOUser(context.Background(), apiOpts, tc.tokenResp)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, user)
			require.NotNil(t, token)
			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)
			require.Equal(t, tc.tokenResp.Data.Payload.Email, user.Email)
			require.Equal(t, tc.tokenResp.Data.Payload.FirstName, user.FirstName)
			require.Equal(t, tc.tokenResp.Data.Payload.LastName, user.LastName)
		})
	}
}

func TestLoginUserSSOService_EndToEndLoginFlow(t *testing.T) {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(t, err)
	cfg, err := config.Get()
	require.NoError(t, err)
	cfg.Auth.SSO.RedirectURL = "https://convoy.example.com/sso/callback"
	require.NoError(t, config.Override(&cfg))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service, ssoServer := provideLoginUserSSOService(ctrl, t)
	defer ssoServer.Close()

	us, _ := service.UserRepo.(*mocks.MockUserRepository)
	us.EXPECT().FindUserByEmail(gomock.Any(), "sso@test.com").
		Times(1).Return(&datastore.User{
		UID:       "user-123",
		FirstName: "SSO",
		LastName:  "User",
		Email:     "sso@test.com",
	}, nil)

	redirectResp, err := service.Run()
	require.NoError(t, err)
	require.NotNil(t, redirectResp)
	require.NotEmpty(t, redirectResp.RedirectURL)

	queryVals := url.Values{
		"token": []string{"test-token-123"},
	}

	tokenResp, err := service.RedeemToken(queryVals)
	require.NoError(t, err)
	require.NotNil(t, tokenResp)
	require.True(t, tokenResp.Status)
	require.Equal(t, "sso@test.com", tokenResp.Data.Payload.Email)

	user, jwtToken, err := service.LoginSSOUser(context.Background(), tokenResp)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, jwtToken)
	require.NotEmpty(t, jwtToken.AccessToken)
	require.NotEmpty(t, jwtToken.RefreshToken)
	require.Equal(t, "sso@test.com", user.Email)
}
