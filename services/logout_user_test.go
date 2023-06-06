package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideLogoutUserService(ctrl *gomock.Controller, t *testing.T, token string) (*LogoutUserService, cache.Cache) {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return &LogoutUserService{
		JWT:      jwt.NewJwt(&config.Auth.Jwt, c),
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Token:    token,
	}, c
}

func TestLogoutUserService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx   context.Context
		user  *datastore.User
		token *models.Token
	}

	type token struct {
		generate    bool
		accessToken bool
	}

	tests := []struct {
		name       string
		args       args
		dbFn       func(u *LogoutUserService, cache cache.Cache)
		wantConfig bool
		wantToken  token
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_logout_user",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "12345"},
				token: &models.Token{},
			},
			dbFn: func(u *LogoutUserService, cache cache.Cache) {
				ca, _ := cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
				ca.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantToken: token{generate: true, accessToken: true},
		},

		{
			name: "should_fail_to_logout_user_with_invalid_access_token",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "12345"},
				token: &models.Token{AccessToken: ulid.Make().String()},
			},
			dbFn: func(u *LogoutUserService, cache cache.Cache) {
				ca, _ := cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to validate token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u, c := provideLogoutUserService(ctrl, t, tc.args.token.AccessToken)

			if tc.dbFn != nil {
				tc.dbFn(u, c)
			}

			if tc.wantToken.generate {
				token, err := u.JWT.GenerateToken(tc.args.user)
				require.Nil(t, err)

				if tc.wantToken.accessToken {
					u.Token = token.AccessToken
				}
			}

			err := u.Run(tc.args.ctx)

			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
