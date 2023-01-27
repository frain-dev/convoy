package jwt

import (
	"context"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJwtRealm_Authenticate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mocks.NewMockUserRepository(ctrl)
	cache, err := cache.NewCache(config.CacheConfiguration{})

	require.Nil(t, err)

	jr := NewJwtRealm(userRepo, &config.JwtRealmOptions{}, cache)

	user := &datastore.User{UID: "123456"}
	token, err := jr.jwt.GenerateToken(user)

	require.Nil(t, err)

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
				AuthenticatedByRealm: jr.GetName(),
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
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type JWT", jr.GetName()),
		},

		{
			name: "should_error_for_invalid_token",
			args: args{
				cred: &auth.Credential{
					Type:  auth.CredentialTypeJWT,
					Token: uuid.NewString(),
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
			dbFn:       nil,
			want:       nil,
			blacklist:  true,
			wantErr:    true,
			wantErrMsg: "invalid token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dbFn != nil {
				tc.dbFn(userRepo)
			}

			if tc.blacklist {
				err := jr.jwt.BlacklistToken(&VerifiedToken{UserID: user.UID, Expiry: 10}, token.AccessToken)
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
