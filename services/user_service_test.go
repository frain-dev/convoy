package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func provideUserService(ctrl *gomock.Controller, t *testing.T) *UserService {
	userRepo := mocks.NewMockUserRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	queue := mocks.NewMockQueuer(ctrl)

	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	userService := NewUserService(userRepo, cache, queue)
	return userService
}

func TestUserService_LoginUser(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx  context.Context
		user *models.LoginUser
	}

	tests := []struct {
		name        string
		args        args
		wantUser    *datastore.User
		dbFn        func(u *UserService)
		wantConfig  bool
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
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
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
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
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "invalid username or password",
		},

		{
			name: "should_not_login_with_invalid_password",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "test@test.com", Password: "12345"},
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
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
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "invalid username or password",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUserService(ctrl, t)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			if tc.wantConfig {
				err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
				require.Nil(t, err)
			}

			user, token, err := u.LoginUser(tc.args.ctx, tc.args.user)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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

func TestUserService_RefreshToken(t *testing.T) {
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
		dbFn        func(u *UserService)
		wantConfig  bool
		wantToken   token
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_refresh_token",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{},
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				ca, _ := u.cache.(*mocks.MockCache)

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
				token: &models.Token{AccessToken: uuid.NewString(), RefreshToken: uuid.NewString()},
			},
			dbFn: func(u *UserService) {
				ca, _ := u.cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
		},

		{
			name: "should_fail_to_refresh_for_invalid_refresh_token",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "123456"},
				token: &models.Token{RefreshToken: uuid.NewString()},
			},
			dbFn: func(u *UserService) {
				ca, _ := u.cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil)
			},
			wantToken:   token{generate: true, accessToken: true},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUserService(ctrl, t)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			if tc.wantToken.generate {
				jwt, err := u.token()
				require.Nil(t, err)

				token, err := jwt.GenerateToken(tc.args.user)
				require.Nil(t, err)

				if tc.wantToken.accessToken {
					tc.args.token.AccessToken = token.AccessToken
				}

				if tc.wantToken.refreshToken {
					tc.args.token.RefreshToken = token.RefreshToken
				}
			}

			token, err := u.RefreshToken(tc.args.ctx, tc.args.token)

			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, token.AccessToken)
			require.NotEmpty(t, token.RefreshToken)
		})
	}
}

func TestUserService_LogoutUser(t *testing.T) {
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
		name        string
		args        args
		dbFn        func(u *UserService)
		wantConfig  bool
		wantToken   token
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{

		{
			name: "should_logout_user",
			args: args{
				ctx:   ctx,
				user:  &datastore.User{UID: "12345"},
				token: &models.Token{},
			},
			dbFn: func(u *UserService) {
				ca, _ := u.cache.(*mocks.MockCache)
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
				token: &models.Token{AccessToken: uuid.NewString()},
			},
			dbFn: func(u *UserService) {
				ca, _ := u.cache.(*mocks.MockCache)
				ca.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUserService(ctrl, t)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			if tc.wantToken.generate {
				jwt, err := u.token()
				require.Nil(t, err)

				token, err := jwt.GenerateToken(tc.args.user)
				require.Nil(t, err)

				if tc.wantToken.accessToken {
					tc.args.token.AccessToken = token.AccessToken
				}
			}

			err := u.LogoutUser(tc.args.token.AccessToken)

			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		user   *datastore.User
		update *models.UpdateUser
	}

	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantUser    *datastore.User
		dbFn        func(u *UserService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_user",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456"},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "test@update.com",
				},
			},
			wantUser: &datastore.User{
				FirstName: "update_user_test",
				LastName:  "update_user_test",
				Email:     "test@update.com",
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)
			},
		},

		{
			name: "should_fail_to_update_user",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456"},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "test@update.com",
				},
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(errors.New("an error occurred while updating user"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while updating user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUserService(ctrl, t)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			user, err := u.UpdateUser(tc.args.ctx, tc.args.update, tc.args.user)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, user.UID)

			require.Equal(t, user.FirstName, tc.wantUser.FirstName)
			require.Equal(t, user.LastName, tc.wantUser.LastName)
			require.Equal(t, user.Email, tc.wantUser.Email)
		})
	}
}

func TestUserService_UpdatePassword(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		user   *datastore.User
		update *models.UpdatePassword
	}

	currentPassword := "123456"
	p := datastore.Password{Plaintext: currentPassword}

	err := p.GenerateHash()
	require.Nil(t, err)

	tests := []struct {
		name        string
		args        args
		wantErr     bool
		dbFn        func(u *UserService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_password",
			args: args{
				ctx: ctx,
				user: &datastore.User{
					UID:      "123456",
					Password: string(p.Hash),
				},
				update: &models.UpdatePassword{
					CurrentPassword:      currentPassword,
					Password:             "123456789",
					PasswordConfirmation: "123456789",
				},
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)
			},
		},

		{
			name: "should_fail_to_update_password",
			args: args{
				ctx: ctx,
				user: &datastore.User{
					UID:      "123456",
					Password: string(p.Hash),
				},
				update: &models.UpdatePassword{
					CurrentPassword:      currentPassword,
					Password:             "123456789",
					PasswordConfirmation: "123456789",
				},
			},
			dbFn: func(u *UserService) {
				us, _ := u.userRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while updating user",
		},

		{
			name: "should_fail_to_update_password_invalid_current_password",
			args: args{
				ctx: ctx,
				user: &datastore.User{
					UID:      "123456",
					Password: string(p.Hash),
				},
				update: &models.UpdatePassword{
					CurrentPassword:      "random",
					Password:             "123456789",
					PasswordConfirmation: "123456789",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "current password is invalid",
		},

		{
			name: "should_fail_to_update_password_invalid_password_confirmation",
			args: args{
				ctx: ctx,
				user: &datastore.User{
					UID:      "123456",
					Password: string(p.Hash),
				},
				update: &models.UpdatePassword{
					CurrentPassword:      currentPassword,
					Password:             "123456789",
					PasswordConfirmation: "12345678",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "password confirmation doesn't match password",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUserService(ctrl, t)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			user, err := u.UpdatePassword(tc.args.ctx, tc.args.update, tc.args.user)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			pa := datastore.Password{Plaintext: tc.args.update.Password, Hash: []byte(user.Password)}
			isMatch, err := pa.Matches()

			require.Nil(t, err)
			require.True(t, isMatch)
		})
	}
}
