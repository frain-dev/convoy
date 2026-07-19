package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideUpdateUserService(ctrl *gomock.Controller, data *models.UpdateUser, user *datastore.User) *UpdateUserService {
	return &UpdateUserService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Queue:    mocks.NewMockQueuer(ctrl),
		BaseURL:  "https://dashboard.example.com",
		Data:     data,
		User:     user,
		Logger:   mocks.NewMockLogger(ctrl),
	}
}

func TestUpdateUserService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		user   *datastore.User
		update *models.UpdateUser
	}

	tests := []struct {
		name         string
		args         args
		wantErr      bool
		wantUser     *datastore.User
		wantVerified bool
		dbFn         func(u *UpdateUserService)
		wantErrMsg   string
	}{
		{
			name: "should_update_user_without_email_change",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", Email: "test@update.com", EmailVerified: true},
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
			wantVerified: true,
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "should_unverify_and_send_verification_on_email_change",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", Email: "old@update.com", EmailVerified: true},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "new@update.com",
				},
			},
			wantUser: &datastore.User{
				FirstName: "update_user_test",
				LastName:  "update_user_test",
				Email:     "new@update.com",
			},
			wantVerified: false,
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)

				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "should_error_for_use_email_not_verified",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", Email: "old@update.com", EmailVerified: false},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "old@update.com",
				},
			},
			dbFn:       func(u *UpdateUserService) {},
			wantErr:    true,
			wantErrMsg: "email has not been verified",
		},
		{
			// Recovery path: a mistyped email leaves the user unverified, so
			// correcting the address must not be blocked by the verified gate.
			name: "should_allow_unverified_user_to_correct_email",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", Email: "typo@update.com", EmailVerified: false},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "correct@update.com",
				},
			},
			wantUser: &datastore.User{
				FirstName: "update_user_test",
				LastName:  "update_user_test",
				Email:     "correct@update.com",
			},
			wantVerified: false,
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)

				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},

		{
			name: "should_fail_to_update_user",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", Email: "test@update.com", EmailVerified: true},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "test@update.com",
				},
			},
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(errors.New("an error occurred while updating user"))

				ml, _ := u.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to update user", "error", gomock.Any()).Times(1)
			},
			wantErr:    true,
			wantErrMsg: "failed to update user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideUpdateUserService(ctrl, tc.args.update, tc.args.user)

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
			require.NotEmpty(t, user.UID)

			require.Equal(t, user.FirstName, tc.wantUser.FirstName)
			require.Equal(t, user.LastName, tc.wantUser.LastName)
			require.Equal(t, user.Email, tc.wantUser.Email)
			require.Equal(t, tc.wantVerified, user.EmailVerified)
			if !tc.wantVerified {
				require.NotEmpty(t, user.EmailVerificationToken)
				require.True(t, user.EmailVerificationExpiresAt.After(user.CreatedAt))
			}
		})
	}
}
