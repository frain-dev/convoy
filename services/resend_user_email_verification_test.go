package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

type stubResendClaimStore struct {
	ok       bool
	claimErr error
	token    string
	released string
}

func (s *stubResendClaimStore) TryClaim(ctx context.Context, userUID string) (bool, string, error) {
	if s.claimErr != nil {
		return false, "", s.claimErr
	}
	if !s.ok {
		return false, "", nil
	}
	if s.token == "" {
		s.token = "claim-token"
	}
	return true, s.token, nil
}

func (s *stubResendClaimStore) Release(ctx context.Context, userUID, token string) error {
	s.released = token
	return nil
}

func provideResendEmailVerificationTokenService(ctrl *gomock.Controller, user *datastore.User, baseURL string, claim ResendClaimStore) *ResendEmailVerificationTokenService {
	return &ResendEmailVerificationTokenService{
		UserRepo:   mocks.NewMockUserRepository(ctrl),
		Queue:      mocks.NewMockQueuer(ctrl),
		ClaimStore: claim,
		BaseURL:    baseURL,
		User:       user,
	}
}

func TestResendEmailVerificationTokenService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		baseURL string
		user    *datastore.User
		claim   *stubResendClaimStore
	}
	tests := []struct {
		name           string
		args           args
		dbFn           func(u *ResendEmailVerificationTokenService)
		wantErr        bool
		wantErrMsg     string
		wantReleaseTok string
	}{
		{
			name: "should_resend_verification_email",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-1", EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(-time.Hour)},
				claim:   &stubResendClaimStore{ok: true, token: "tok-1"},
			},
			dbFn: func(u *ResendEmailVerificationTokenService) {
				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_resend_while_previous_token_unexpired",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-2", EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(90 * time.Minute)},
				claim:   &stubResendClaimStore{ok: true, token: "tok-2"},
			},
			dbFn: func(u *ResendEmailVerificationTokenService) {
				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

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
				user:    &datastore.User{UID: "user-3", EmailVerified: true, EmailVerificationExpiresAt: time.Now().Add(-time.Hour)},
				claim:   &stubResendClaimStore{ok: true},
			},
			wantErr:    true,
			wantErrMsg: "user email already verified",
		},
		{
			name: "should_error_for_resend_cooldown",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-4", EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(emailVerificationTokenTTL - 5*time.Second)},
				claim:   &stubResendClaimStore{ok: true},
			},
			wantErr:    true,
			wantErrMsg: "please wait before requesting another verification email",
		},
		{
			name: "should_error_when_claim_held",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-5", EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(90 * time.Minute)},
				claim:   &stubResendClaimStore{ok: false},
			},
			wantErr:    true,
			wantErrMsg: "please wait before requesting another verification email",
		},
		{
			name: "should_release_claim_when_update_fails",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-6", EmailVerified: false, EmailVerificationExpiresAt: time.Now().Add(90 * time.Minute)},
				claim:   &stubResendClaimStore{ok: true, token: "tok-6"},
			},
			dbFn: func(u *ResendEmailVerificationTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("db down"))
			},
			wantErr:        true,
			wantErrMsg:     "failed to update user",
			wantReleaseTok: "tok-6",
		},
		{
			name: "should_restore_token_and_release_claim_when_queue_fails",
			args: args{
				ctx:     ctx,
				baseURL: "localhost",
				user:    &datastore.User{UID: "user-7", EmailVerified: false, EmailVerificationToken: "prev-token", EmailVerificationExpiresAt: time.Now().Add(90 * time.Minute)},
				claim:   &stubResendClaimStore{ok: true, token: "tok-7"},
			},
			dbFn: func(u *ResendEmailVerificationTokenService) {
				us, _ := u.UserRepo.(*mocks.MockUserRepository)
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, user *datastore.User) error {
					require.NotEqual(t, "prev-token", user.EmailVerificationToken)
					return nil
				})
				us.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, user *datastore.User) error {
					require.Equal(t, "prev-token", user.EmailVerificationToken)
					return nil
				})

				q, _ := u.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("queue down"))
			},
			wantErr:        true,
			wantErrMsg:     "failed to queue user verification email",
			wantReleaseTok: "tok-7",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			u := provideResendEmailVerificationTokenService(ctrl, tc.args.user, tc.args.baseURL, tc.args.claim)

			if tc.dbFn != nil {
				tc.dbFn(u)
			}

			err := u.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
			} else {
				require.Nil(t, err)
			}
			if tc.args.claim != nil {
				require.Equal(t, tc.wantReleaseTok, tc.args.claim.released)
			}
		})
	}
}
