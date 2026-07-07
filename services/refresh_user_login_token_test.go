package services

import (
	"context"
	"errors"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func provideRefreshTokenService(ctrl *gomock.Controller, t *testing.T, data *models.Token) (*RefreshTokenService, cache.Cache) {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return NewRefreshTokenService(
		mocks.NewMockUserRepository(ctrl),
		mocks.NewMockOrganisationMemberRepository(ctrl),
		jwt.NewJwt(&config.Auth.Jwt, c),
		mocks.NewMockLicenser(ctrl),
		data,
		log.New("convoy-test", log.LevelError),
	), c
}

func TestRefreshTokenService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx   context.Context
		user  *datastore.User
		token *models.Token
	}

	type token struct {
		generate     bool
		accessToken  bool
		refreshToken bool
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(u *RefreshTokenService, cache cache.Cache)
		wantConfig  bool
		wantToken   token
		wantErr     bool
		wantErrMsg  string
		wantErrCode string
	}{
		{
			name: "should_refresh_token",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{},
			},
			dbFn: func(u *RefreshTokenService, cache cache.Cache) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				ca, _ := cache.(*mocks.MockCache)
				lc, _ := u.Licenser.(*mocks.MockLicenser)

				us.EXPECT().FindUserByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{UID: "123456"}, nil)
				lc.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
				ca.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantConfig: true,
			wantToken:  token{generate: true, accessToken: true, refreshToken: true},
		},

		{
			name: "should_fail_to_refresh_when_license_expired_for_non_admin",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{},
			},
			dbFn: func(u *RefreshTokenService, cache cache.Cache) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				om, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				ca, _ := cache.(*mocks.MockCache)
				lc, _ := u.Licenser.(*mocks.MockLicenser)

				us.EXPECT().FindUserByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{UID: "123456"}, nil)
				// Single-user mode: not multi-user, an instance admin exists, and
				// this user is not it, so refresh must be denied.
				lc.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(false, nil)
				om.EXPECT().CountInstanceAdminUsers(gomock.Any()).Times(1).Return(int64(1), nil)
				om.EXPECT().IsFirstInstanceAdmin(gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
			},
			wantToken:   token{generate: true, accessToken: true, refreshToken: true},
			wantErr:     true,
			wantErrMsg:  "License expired",
			wantErrCode: ErrCodeLicenseExpired,
		},

		{
			name: "should_return_internal_error_when_license_eval_fails",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{},
			},
			dbFn: func(u *RefreshTokenService, cache cache.Cache) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				ca, _ := cache.(*mocks.MockCache)
				lc, _ := u.Licenser.(*mocks.MockLicenser)

				us.EXPECT().FindUserByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{UID: "123456"}, nil)
				// License evaluation hits a transient failure, which is a
				// server-side error and must not be reported as a bad token or a
				// definitive "license expired" denial.
				lc.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(false, errors.New("license cache unavailable"))
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
			},
			wantToken:   token{generate: true, accessToken: true, refreshToken: true},
			wantErr:     true,
			wantErrMsg:  "failed to evaluate license access",
			wantErrCode: ErrCodeInternal,
		},

		{
			name: "should_fail_to_refresh_for_invalid_access_token",
			args: args{
				ctx:   ctx,
				token: &models.Token{AccessToken: ulid.Make().String(), RefreshToken: ulid.Make().String()},
			},
			dbFn: func(u *RefreshTokenService, cache cache.Cache) {
				ca, _ := cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:    true,
			wantErrMsg: "token contains an invalid number of segments",
		},

		{
			name: "should_fail_to_refresh_for_invalid_refresh_token",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{RefreshToken: ulid.Make().String()},
			},
			dbFn: func(u *RefreshTokenService, cache cache.Cache) {
				ca, _ := cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
			},
			wantToken:  token{generate: true, accessToken: true},
			wantErrMsg: "token contains an invalid number of segments",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u, c := provideRefreshTokenService(ctrl, t, tc.args.token)

			if tc.dbFn != nil {
				tc.dbFn(u, c)
			}

			if tc.wantToken.generate {
				token, err := u.JWT.GenerateToken(tc.args.user)
				require.Nil(t, err)

				if tc.wantToken.accessToken {
					tc.args.token.AccessToken = token.AccessToken
				}

				if tc.wantToken.refreshToken {
					tc.args.token.RefreshToken = token.RefreshToken
				}
			}

			token, err := u.Run(tc.args.ctx)

			if tc.wantErr {
				require.NotNil(t, err)
				require.Contains(t, err.(*ServiceError).Error(), tc.wantErrMsg)
				if tc.wantErrCode != "" {
					require.Equal(t, tc.wantErrCode, err.(*ServiceError).Code)
				}
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)
		})
	}
}
