package services

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideResendEmailVerificationTokenService(ctrl *gomock.Controller, user *datastore.User, baseURL string) *ResendEmailVerificationTokenService {
	return &ResendEmailVerificationTokenService{
		UserRepo: mocks.NewMockUserRepository(ctrl),
		Queue:    mocks.NewMockQueuer(ctrl),
		BaseURL:  baseURL,
		User:     user,
	}
}

func TestResendEmailVerificationTokenService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		baseURL string
		user    *datastore.User
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(u *ResendEmailVerificationTokenService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_resend_verification_email",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(-time.Hour)},
			},
			dbFn: func(u *ResendEmailVerificationTokenService) {
				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_error_for_email_verifiied",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{EmailVerified: true, EmailVerificationExpiresAt: time.Now().Add(-time.Hour)},
			},
			wantErr:    true,
			wantErrMsg: "user email already verified",
		},
		{
			name: "should_error_for_previous_token_not_expired",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(time.Hour)},
			},
			wantErr:    true,
			wantErrMsg: "old verification token is still valid",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideResendEmailVerificationTokenService(ctrl, tc.args.user, tc.args.baseURL)

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
