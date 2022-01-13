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
	mockApiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)

	nr := NewNativeRealm(mockApiKeyRepo)

	type args struct {
		cred *auth.Credential
	}
	tests := []struct {
		name       string
		args       args
		nFn        func(apiKeyRepo *mocks.MockAPIKeyRepository)
		want       *auth.AuthenticatedUser
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_authenticate_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: nr.GetName(),
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
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
					Type: auth.CredentialTypeBasic,
				},
			},
			nFn:        nil,
			want:       nil,
			wantErr:    true,
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type BEARER", nr.GetName()),
		},
		{
			name: "should_error_for_revoked_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					DeletedAt: primitive.NewDateTimeFromTime(time.Now()),
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
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&convoy.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:   auth.RoleUIAdmin,
						Groups: []string{"paystack"},
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
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
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
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
