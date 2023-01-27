package native

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNativeRealm_Authenticate(t *testing.T) {
	type args struct {
		cred *auth.Credential
	}
	tests := []struct {
		name       string
		args       args
		nFn        func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository)
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
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "native_realm",
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
				Role: auth.Role{
					Type:    auth.RoleAdmin,
					Project: "paystack",
				},
				APIKey: &datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "should_authenticate_personal_apiKey_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)

				userRepo.EXPECT().FindUserByID(gomock.Any(), "1234").Times(1).Return(&datastore.User{UID: "1234"}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "native_realm",
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
				Role: auth.Role{
					Type:    auth.RoleAdmin,
					Project: "paystack",
				},
				Metadata: &datastore.User{UID: "1234"},
				User:     &datastore.User{UID: "1234"},
				APIKey: &datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_failed_to_fined_user",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)

				userRepo.EXPECT().FindUserByID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch user: failed",
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
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type BEARER", "native_realm"),
		},
		{
			name: "should_error_for_revoked_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					DeletedAt: util.NewDateTime(),
					ExpiresAt: 0,
					CreatedAt: 0,
				}, nil)
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "api key has been revoked",
		},
		{
			name: "should_error_for_invalid_key_format",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "abcd",
				},
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "invalid api key format",
		},
		{
			name: "should_error_for_expired_key",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
				apiKeyRepo.EXPECT().
					FindAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: primitive.NewDateTimeFromTime(time.Now().Add(time.Second * -10)),
					DeletedAt: nil,
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
			nFn: func(apiKeyRepo *mocks.MockAPIKeyRepository, userRepo *mocks.MockUserRepository) {
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockApiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			mockUserRepo := mocks.NewMockUserRepository(ctrl)

			nr := NewNativeRealm(mockApiKeyRepo, mockUserRepo)
			if tt.nFn != nil {
				tt.nFn(mockApiKeyRepo, mockUserRepo)
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
