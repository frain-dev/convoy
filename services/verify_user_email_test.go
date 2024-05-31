package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideVerifyEmailService(ctrl *gomock.Controller, token string) *VerifyEmailService {
	return &VerifyEmailService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Token:    token,
	}
}

func TestVerifyEmailService_Run(t *testing.T) {
	type args struct {
		ctx   context.Context
		token string
	}
	ctx := context.Background()

	tests := []struct {
		name       string
		dbFn       func(u *VerifyEmailService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_verify_email",
			dbFn: func(u *VerifyEmailService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)

				user := &datastore.User{
					UID:                        "abc",
					EmailVerificationToken:     "12345",
					EmailVerificationExpiresAt: time.Now().Add(time.Hour),
				}

				us.EXPECT().FindUserByEmailVerificationToken(gomock.Any(), "12345").Times(1).Return(
					user,
					nil,
				)

				u1 := *user
				u1.EmailVerified = true
				us.EXPECT().UpdateUser(gomock.Any(), &u1).Times(1).Return(nil)
			},
			args: args{
				ctx:   ctx,
				token: "12345",
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_user",
			dbFn: func(u *VerifyEmailService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)

				us.EXPECT().FindUserByEmailVerificationToken(gomock.Any(), "12345").Times(1).Return(
					nil,
					datastore.ErrUserNotFound,
				)
			},
			args: args{
				ctx:   ctx,
				token: "12345",
			},
			wantErr:    true,
			wantErrMsg: "invalid password reset token",
		},
		{
			name: "should_fail_to_find_user",
			dbFn: func(u *VerifyEmailService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)

				us.EXPECT().FindUserByEmailVerificationToken(gomock.Any(), "12345").Times(1).Return(
					nil,
					errors.New("failed to find user"),
				)
			},
			args: args{
				ctx:   ctx,
				token: "12345",
			},
			wantErr:    true,
			wantErrMsg: "failed to find user",
		},
		{
			name: "should_fail_to_update_user",
			dbFn: func(u *VerifyEmailService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)

				user := &datastore.User{
					UID:                        "abc",
					EmailVerificationToken:     "12345",
					EmailVerificationExpiresAt: time.Now().Add(time.Hour),
				}

				us.EXPECT().FindUserByEmailVerificationToken(gomock.Any(), "12345").Times(1).Return(
					user,
					nil,
				)

				u1 := *user
				u1.EmailVerified = true
				us.EXPECT().UpdateUser(gomock.Any(), &u1).Times(1).Return(errors.New("failed to update user"))
			},
			args: args{
				ctx:   ctx,
				token: "12345",
			},
			wantErr:    true,
			wantErrMsg: "failed to update user",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideVerifyEmailService(ctrl, tc.args.token)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			err := u.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
