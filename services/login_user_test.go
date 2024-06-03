package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideLoginUserService(ctrl *gomock.Controller, t *testing.T, loginUser *models.LoginUser) *LoginUserService {
	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return &LoginUserService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Cache:    c,
		JWT:      jwt.NewJwt(&config.Auth.Jwt, c),
		Data:     loginUser,
	}
}

func TestLoginUserService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx  context.Context
		user *models.LoginUser
	}

	tests := []struct {
		name       string
		args       args
		wantUser   *datastore.User
		dbFn       func(u *LoginUserService)
		wantConfig bool
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_login_user_with_valid_credentials",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "test@test.com", Password: "123456"},
			},
			wantUser: &datastore.User{
				UID:       "12345",
				FirstName: "test",
				LastName:  "test",
				Email:     "test@test.com",
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				p := &datastore.Password{Plaintext: "123456"}
				err := p.GenerateHash()
				if err != nil {
					t.Fatal(err)
				}

				us.EXPECT().FindUserByEmail(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{
					UID:       "12345",
					FirstName: "test",
					LastName:  "test",
					Email:     "test@test.com",
					Password:  string(p.Hash),
				}, nil)
			},
			wantConfig: true,
		},

		{
			name: "should_not_login_with_invalid_username",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "invalid@test.com", Password: "123456"},
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantErr:    true,
			wantErrMsg: "invalid username or password",
		},

		{
			name: "should_not_login_with_invalid_password",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "test@test.com", Password: "12345"},
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				p := &datastore.Password{Plaintext: "123456"}
				err := p.GenerateHash()
				if err != nil {
					t.Fatal(err)
				}

				us.EXPECT().FindUserByEmail(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{
					UID:       "12345",
					FirstName: "test",
					LastName:  "test",
					Email:     "test@test.com",
					Password:  string(p.Hash),
				}, nil)
			},
			wantErr:    true,
			wantErrMsg: "invalid username or password",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantConfig {
				err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
				require.Nil(t, err)
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideLoginUserService(ctrl, t, tc.args.user)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			user, token, err := u.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, user.UID)
			require.NotEmpty(t, user.FirstName)

			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)

			require.Equal(t, user.FirstName, tc.wantUser.FirstName)
			require.Equal(t, user.LastName, tc.wantUser.LastName)
			require.Equal(t, user.Email, tc.wantUser.Email)
		})
	}
}
