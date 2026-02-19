package jwt

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func TestJwtRealm_Authenticate(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	newCache := mcache.NewMemoryCache()
	jr := NewJwtRealm(userRepo, &config.JwtRealmOptions{}, newCache)

	user := &datastore.User{UID: "123456"}
	token, err := jr.jwt.GenerateToken(user)
	require.Nil(t, err)

	ctrl.Finish()

	type args struct {
		cred *auth.Credential
	}

	tests := []struct {
		name       string
		args       args
		dbFn       func(userRepo *mocks.MockUserRepository)
		want       *auth.AuthenticatedUser
		blacklist  bool
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_authenticate_successfully",
			args: args{
				cred: &auth.Credential{
					Type:  auth.CredentialTypeJWT,
					Token: token.AccessToken,
				},
			},
			dbFn: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().FindUserByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.User{
					UID:       "123456",
					FirstName: "test",
					LastName:  "test",
				}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: auth.JWTRealmName,
				Credential: auth.Credential{
					Type:  auth.CredentialTypeJWT,
					Token: token.AccessToken,
				},
				Metadata: &datastore.User{
					UID:       "123456",
					FirstName: "test",
					LastName:  "test",
				},
				User: &datastore.User{
					UID:       "123456",
					FirstName: "test",
					LastName:  "test",
				},
				Role: auth.Role{},
			},
		},

		{
			name: "should_error_for_wrong_cred_type",
			args: args{
				cred: &auth.Credential{
					Type: auth.CredentialTypeAPIKey,
				},
			},
			dbFn:       nil,
			want:       nil,
			wantErr:    true,
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type JWT", auth.JWTRealmName),
		},

		{
			name: "should_error_for_invalid_token",
			args: args{
				cred: &auth.Credential{
					Type:  auth.CredentialTypeJWT,
					Token: ulid.Make().String(),
				},
			},
			dbFn:       nil,
			want:       nil,
			wantErr:    true,
			wantErrMsg: "invalid token",
		},

		{
			name: "should_error_for_blacklisted_token",
			args: args{
				cred: &auth.Credential{
					Type:  auth.CredentialTypeJWT,
					Token: token.AccessToken,
				},
			},
			dbFn: func(userRepo *mocks.MockUserRepository) {
			},
			want:       nil,
			blacklist:  true,
			wantErr:    true,
			wantErrMsg: "invalid token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			userRepo := mocks.NewMockUserRepository(ctrl)
			newCache := mcache.NewMemoryCache()
			jr := NewJwtRealm(userRepo, &config.JwtRealmOptions{}, newCache)

			if tc.dbFn != nil {
				tc.dbFn(userRepo)
			}

			if tc.blacklist {
				err := jr.jwt.BlacklistToken(&VerifiedToken{UserID: user.UID, Expiry: time.Now().Add(time.Minute).Unix()}, token.AccessToken)
				require.Nil(t, err)
			}

			got, err := jr.Authenticate(context.Background(), tc.args.cred)
			if tc.wantErr {
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
