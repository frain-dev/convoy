package native

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNativeRealm_Authenticate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockApiKeyRepo := mocks.NewMockAPIKeyRepo(ctrl)

	nr := NewNativeRealm(mockApiKeyRepo)

	type args struct {
		cred *auth.Credential
	}
	tests := []struct {
		name       string
		args       args
		nFn        func(apiKeyRepo *mocks.MockAPIKeyRepo)
		want       *auth.AuthenticatedUser
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_authenticate_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcde",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepo) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByHash(gomock.Any(), gomock.AssignableToTypeOf("")).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					Hash:      "abcd",
					Revoked:   false,
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: nr.GetName(),
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcde",
				},
				Role: auth.Role{
					Type:   auth.RoleUIAdmin,
					Groups: []string{"paystack"},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_cred_type",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeBasic,
					APIKey: "abcde",
				},
			},
			nFn:        nil,
			want:       nil,
			wantErr:    true,
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type API_KEY", nr.GetName()),
		},
		{
			name: "should_error_for_revoked_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcde",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepo) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByHash(gomock.Any(), gomock.AssignableToTypeOf("")).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					Hash:      "abcd",
					Revoked:   true,
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "api key has been revoked",
		},
		{
			name: "should_error_for_expired_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcde",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepo) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByHash(gomock.Any(), gomock.AssignableToTypeOf("")).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					Hash:      "abcd",
					Revoked:   false,
					ExpiresAt: primitive.NewDateTimeFromTime(time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC)),
					CreatedAt: 0,
				}, nil)
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "api key has expired",
		},
		{
			name: "should_error_failure_to_find_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcde",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepo) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByHash(gomock.Any(), gomock.AssignableToTypeOf("")).
					Times(1).Return(nil, errors.New("no documents in result"))
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "failed to hash api key: no documents in result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nFn != nil {
				tt.nFn(mockApiKeyRepo)
			}

			got, err := nr.Authenticate(context.Background(), tt.args.cred)
			if tt.wantErr {
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
