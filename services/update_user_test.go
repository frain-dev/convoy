package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateUserService(ctrl *gomock.Controller, data *models.UpdateUser, user *datastore.User) *UpdateUserService {
	return &UpdateUserService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Data:     data,
		User:     user,
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
		name       string
		args       args
		wantErr    bool
		wantUser   *datastore.User
		dbFn       func(u *UpdateUserService)
		wantErrMsg string
	}{
		{
			name: "should_update_user",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", EmailVerified: true},
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
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "should_error_for_use_email_not_verified",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", EmailVerified: false},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "test@update.com",
				},
			},
			dbFn:       func(u *UpdateUserService) {},
			wantErr:    true,
			wantErrMsg: "email has not been verified",
		},

		{
			name: "should_fail_to_update_user",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "123456", EmailVerified: true},
				update: &models.UpdateUser{
					FirstName: "update_user_test",
					LastName:  "update_user_test",
					Email:     "test@update.com",
				},
			},
			dbFn: func(u *UpdateUserService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(errors.New("an error occurred while updating user"))
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
		})
	}
}
