package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideRegisterUserService(ctrl *gomock.Controller, t *testing.T, baseUrl string, loginUser *models.RegisterUser) *RegisterUserService {
	config, err := config.Get()
	require.NoError(t, err)

	c := mocks.NewMockCache(ctrl)
	return &RegisterUserService{
		UserRepo:      mocks.NewMockUserRepository(ctrl),
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		Queue:         mocks.NewMockQueuer(ctrl),
		JWT:           jwt.NewJwt(&config.Auth.Jwt, c),
		ConfigService: &ConfigService{
			configRepo: mocks.NewMockConfigurationRepository(ctrl),
		},
		BaseURL: baseUrl,
		Data:    loginUser,
	}
}

func TestRegisterUserService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx  context.Context
		user *models.RegisterUser
	}

	tests := []struct {
		name       string
		args       args
		wantUser   *datastore.User
		dbFn       func(u *RegisterUserService)
		wantConfig bool
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "should_register_user",
			wantConfig: true,
			args: args{
				ctx: ctx,
				user: &models.RegisterUser{
					FirstName:        "test",
					LastName:         "test",
					Email:            "test@test.com",
					Password:         "123456",
					OrganisationName: "test",
				},
			},
			wantUser: &datastore.User{
				UID:       "12345",
				FirstName: "test",
				LastName:  "test",
				Email:     "test@test.com",
			},
			dbFn: func(u *RegisterUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				configRepo, _ := u.ConfigService.configRepo.(*mocks.MockConfigurationRepository)
				orgRepo, _ := u.OrgRepo.(*mocks.MockOrganisationRepository)
				orgMemberRepo, _ := u.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				queue, _ := u.Queue.(*mocks.MockQueuer)

				configRepo.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{
					UID:             "12345",
					IsSignupEnabled: true,
				}, nil)

				us.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				orgRepo.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).Times(1).Return(nil)
				orgMemberRepo.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				queue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},

		{
			name:       "should_not_register_user_when_registration_is_not_allowed",
			wantConfig: true,
			args: args{
				ctx: ctx,
				user: &models.RegisterUser{
					FirstName:        "test",
					LastName:         "test",
					Email:            "test@test.com",
					Password:         "123456",
					OrganisationName: "test",
				},
			},
			dbFn: func(u *RegisterUserService) {
				configRepo, _ := u.ConfigService.configRepo.(*mocks.MockConfigurationRepository)
				configRepo.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{
					UID:             "12345",
					IsSignupEnabled: false,
				}, nil)
			},
			wantErr:    true,
			wantErrMsg: "user registration is disabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			if tc.wantConfig {
				err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
				require.Nil(t, err)
			}

			u := provideRegisterUserService(ctrl, t, "localhost", tc.args.user)

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
