package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideRefreshTokenService(ctrl *gomock.Controller, t *testing.T, data *models.Token) (*RefreshTokenService, cache.Cache) {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return &RefreshTokenService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		JWT:      jwt.NewJwt(&config.Auth.Jwt, c),
		Data:     data,
	}, c
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
		name       string
		args       args
		dbFn       func(u *RefreshTokenService, cache cache.Cache)
		wantConfig bool
		wantToken  token
		wantErr    bool
		wantErrMsg string
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

				us.EXPECT().FindUserByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{UID: "123456"}, nil)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
				ca.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantConfig: true,
			wantToken:  token{generate: true, accessToken: true, refreshToken: true},
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
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)
		})
	}
}
