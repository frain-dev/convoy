package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideLoginUserService(ctrl *gomock.Controller, t *testing.T, loginUser *models.LoginUser) *LoginUserService {
	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return &LoginUserService{
		UserRepo:      mocks.NewMockUserRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		Cache:         c,
		JWT:           jwt.NewJwt(&config.Auth.Jwt, c),
		Data:          loginUser,
		Licenser:      mocks.NewMockLicenser(ctrl),
	}
}

func hashTestPassword(t *testing.T, plaintext string) string {
	t.Helper()

	p := &datastore.Password{Plaintext: plaintext}
	err := p.GenerateHash()
	require.NoError(t, err)

	return string(p.Hash)
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
				licenser, _ := u.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)
			},
			wantConfig: true,
		},

		{
			name: "should_login_first_orphan_user_in_single_user_mode",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "edited-first-user@test.com", Password: "default"},
			},
			wantUser: &datastore.User{
				UID:       "first-user",
				FirstName: "first",
				LastName:  "user",
				Email:     "edited-first-user@test.com",
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				orgMembers, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				licenser, _ := u.Licenser.(*mocks.MockLicenser)

				firstUser := &datastore.User{
					UID:       "first-user",
					FirstName: "first",
					LastName:  "user",
					Email:     "edited-first-user@test.com",
					Password:  hashTestPassword(t, "default"),
				}

				us.EXPECT().FindUserByEmail(gomock.Any(), firstUser.Email).Times(1).Return(firstUser, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(false, nil)
				orgMembers.EXPECT().CountInstanceAdminUsers(gomock.Any()).Times(1).Return(int64(0), nil)
				orgMembers.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), firstUser.UID, gomock.Any()).Times(2).Return([]datastore.Organisation{}, datastore.PaginationData{}, nil)
				us.EXPECT().CountUsers(gomock.Any()).Times(1).Return(int64(2), nil)
				us.EXPECT().FindFirstUser(gomock.Any()).Times(1).Return(firstUser, nil)
			},
			wantConfig: true,
		},

		{
			name: "should_not_login_orphan_user_when_not_first_user_in_single_user_mode",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "orphan@test.com", Password: "default"},
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				orgMembers, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				licenser, _ := u.Licenser.(*mocks.MockLicenser)

				orphanUser := &datastore.User{
					UID:       "orphan-user",
					FirstName: "orphan",
					LastName:  "user",
					Email:     "orphan@test.com",
					Password:  hashTestPassword(t, "default"),
				}
				firstUser := &datastore.User{
					UID:   "first-user",
					Email: "first@test.com",
				}

				us.EXPECT().FindUserByEmail(gomock.Any(), orphanUser.Email).Times(1).Return(orphanUser, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(false, nil)
				orgMembers.EXPECT().CountInstanceAdminUsers(gomock.Any()).Times(1).Return(int64(0), nil)
				orgMembers.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), orphanUser.UID, gomock.Any()).Times(2).Return([]datastore.Organisation{}, datastore.PaginationData{}, nil)
				us.EXPECT().CountUsers(gomock.Any()).Times(1).Return(int64(2), nil)
				us.EXPECT().FindFirstUser(gomock.Any()).Times(1).Return(firstUser, nil)
			},
			wantErr:    true,
			wantErrMsg: "This instance does not allow your account to sign in under the current license. Sign in as an instance administrator, enable multi-user licensing, or contact your administrator.",
		},

		{
			name: "should_not_login_first_user_when_user_has_memberships",
			args: args{
				ctx:  ctx,
				user: &models.LoginUser{Username: "first@test.com", Password: "default"},
			},
			dbFn: func(u *LoginUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				orgMembers, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				licenser, _ := u.Licenser.(*mocks.MockLicenser)

				firstUser := &datastore.User{
					UID:       "first-user",
					FirstName: "first",
					LastName:  "user",
					Email:     "first@test.com",
					Password:  hashTestPassword(t, "default"),
				}

				us.EXPECT().FindUserByEmail(gomock.Any(), firstUser.Email).Times(1).Return(firstUser, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(false, nil)
				orgMembers.EXPECT().CountInstanceAdminUsers(gomock.Any()).Times(1).Return(int64(0), nil)
				orgMembers.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), firstUser.UID, gomock.Any()).Times(2).Return([]datastore.Organisation{{UID: "org-1"}}, datastore.PaginationData{}, nil)
				orgMembers.EXPECT().FetchOrganisationMemberByUserID(gomock.Any(), firstUser.UID, "org-1").Times(1).Return(&datastore.OrganisationMember{
					UserID: firstUser.UID,
					Role:   auth.Role{Type: auth.RoleProjectViewer},
				}, nil)
				us.EXPECT().CountUsers(gomock.Any()).Times(1).Return(int64(2), nil)
			},
			wantErr:    true,
			wantErrMsg: "This instance does not allow your account to sign in under the current license. Sign in as an instance administrator, enable multi-user licensing, or contact your administrator.",
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
