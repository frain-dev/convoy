package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideResetPasswordService(ctrl *gomock.Controller, token string, data *models.ResetPassword) *ResetPasswordService {
	return &ResetPasswordService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Token:    token,
		Data:     data,
	}
}

func TestResetPasswordService_Run(t *testing.T) {
	ctx := context.Background()
	resetEx := time.Now().Add(time.Hour)
	type args struct {
		ctx   context.Context
		Token string
		Data  *models.ResetPassword
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(u *ResetPasswordService)
		wantUser   *datastore.User
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_reset_password",
			args: args{
				ctx:   ctx,
				Token: "123456789",
				Data: &models.ResetPassword{
					Password:             "password1",
					PasswordConfirmation: "password1",
				},
			},
			dbFn: func(u *ResetPasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByToken(gomock.Any(), "123456789").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com", ResetPasswordToken: "123456789", ResetPasswordExpiresAt: resetEx},
					nil,
				)

				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantUser: &datastore.User{
				UID:                    "1234",
				Email:                  "test@email.com",
				ResetPasswordToken:     "123456789",
				ResetPasswordExpiresAt: resetEx,
			},
		},
		{
			name: "should_error_for_expired_token",
			args: args{
				ctx:   ctx,
				Token: "123456789",
				Data: &models.ResetPassword{
					Password:             "password1",
					PasswordConfirmation: "password1",
				},
			},
			dbFn: func(u *ResetPasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByToken(gomock.Any(), "123456789").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com", ResetPasswordToken: "123456789", ResetPasswordExpiresAt: resetEx.Add(-2 * time.Hour)},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "password reset token has expired",
		},
		{
			name: "should_error_for_mismatched_password_confirmation",
			args: args{
				ctx:   ctx,
				Token: "123456789",
				Data: &models.ResetPassword{
					Password:             "password1",
					PasswordConfirmation: "password2",
				},
			},
			dbFn: func(u *ResetPasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByToken(gomock.Any(), "123456789").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com", ResetPasswordToken: "123456789", ResetPasswordExpiresAt: resetEx},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "password confirmation doesn't match password",
		},
		{
			name: "should_fail_to_update_user",
			args: args{
				ctx:   ctx,
				Token: "123456789",
				Data: &models.ResetPassword{
					Password:             "password1",
					PasswordConfirmation: "password1",
				},
			},
			dbFn: func(u *ResetPasswordService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByToken(gomock.Any(), "123456789").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com", ResetPasswordToken: "123456789", ResetPasswordExpiresAt: resetEx},
					nil,
				)

				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating user",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideResetPasswordService(ctrl, tt.args.Token, tt.args.Data)

			if tt.dbFn != nil {
				tt.dbFn(u)
			}

			user, err := u.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEqual(t, &tt.wantUser.Password, user.Password)
			user.Password = ""
			require.Equal(t, tt.wantUser, user)
		})
	}
}
