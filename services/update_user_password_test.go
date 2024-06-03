package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdatePasswordService(ctrl *gomock.Controller, data *models.UpdatePassword, user *datastore.User) *UpdatePasswordService {
	return &UpdatePasswordService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Data:     data,
		User:     user,
	}
}

func TestUpdatePasswordService_Run(t *testing.T) {
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
		name       string
		args       args
		wantErr    bool
		dbFn       func(u *UpdatePasswordService)
		wantErrMsg string
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
			dbFn: func(u *UpdatePasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
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
			dbFn: func(u *UpdatePasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating user",
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
			wantErr:    true,
			wantErrMsg: "current password is invalid",
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
			wantErr:    true,
			wantErrMsg: "password confirmation doesn't match password",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUpdatePasswordService(ctrl, tc.args.update, tc.args.user)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			user, err := u.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
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
