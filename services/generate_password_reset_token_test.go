package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/datastore"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
)

func provideGeneratePasswordResetTokenService(ctrl *gomock.Controller, baseURL string, data *models.ForgotPassword) *GeneratePasswordResetTokenService {
	return &GeneratePasswordResetTokenService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Queue:    mocks.NewMockQueuer(ctrl),
		BaseURL:  baseURL,
		Data:     data,
	}
}

func TestGeneratePasswordResetTokenService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		BaseURL string
		Data    *models.ForgotPassword
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(u *GeneratePasswordResetTokenService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_generate_reset_token",
			args: args{
				ctx:     ctx,
				BaseURL: "localhost",
				Data: &models.ForgotPassword{
					Email: "test@email.com",
				},
			},
			dbFn: func(u *GeneratePasswordResetTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com"},
					nil,
				)

				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.EmailProcessor, convoy.DefaultQueue, gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_user_not_found",
			args: args{
				ctx:     ctx,
				BaseURL: "localhost",
				Data: &models.ForgotPassword{
					Email: "test@email.com",
				},
			},
			dbFn: func(u *GeneratePasswordResetTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					nil,
					datastore.ErrUserNotFound,
				)
			},
			wantErr:    true,
			wantErrMsg: "an account with this email does not exist",
		},
		{
			name: "should_fail_to_find_user",
			args: args{
				ctx:     ctx,
				BaseURL: "localhost",
				Data: &models.ForgotPassword{
					Email: "test@email.com",
				},
			},
			dbFn: func(u *GeneratePasswordResetTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to find user by email",
		},
		{
			name: "should_fail_to_update_user",
			args: args{
				ctx:     ctx,
				BaseURL: "localhost",
				Data: &models.ForgotPassword{
					Email: "test@email.com",
				},
			},
			dbFn: func(u *GeneratePasswordResetTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com"},
					nil,
				)

				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to update user",
		},
		{
			name: "should_generate_reset_token",
			args: args{
				ctx:     ctx,
				BaseURL: "localhost",
				Data: &models.ForgotPassword{
					Email: "test@email.com",
				},
			},
			dbFn: func(u *GeneratePasswordResetTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{UID: "1234", Email: "test@email.com"},
					nil,
				)

				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.EmailProcessor, convoy.DefaultQueue, gomock.Any()).Times(1).Return(errors.New("failed to write to queue"))
			},
			wantErr:    true,
			wantErrMsg: "failed to write to queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideGeneratePasswordResetTokenService(ctrl, tt.args.BaseURL, tt.args.Data)

			if tt.dbFn != nil {
				tt.dbFn(u)
			}

			err := u.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
