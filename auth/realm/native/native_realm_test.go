package native

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xdg-go/pbkdf2"
	"go.uber.org/mock/gomock"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func TestNativeRealm_Authenticate(t *testing.T) {
	type args struct {
		cred *auth.Credential
	}
	tests := []struct {
		name       string
		args       args
		nFn        func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository)
		want       *auth.AuthenticatedUser
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_not_authenticate_portal_link_tokens",
			args: args{
				cred: &auth.Credential{
					Type:  auth.CredentialTypeToken,
					Token: "C8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "native_realm",
				Credential: auth.Credential{
					Type:  auth.CredentialTypeToken,
					Token: "C8oU2G7dA75BWrHfFYYvrash",
				},
				PortalLink: &datastore.PortalLink{
					UID:       "abcd",
					Token:     "C8oU2G7dA75BWrHfFYYvrash",
					CreatedAt: time.Time{},
				},
			},
			wantErr:    true,
			wantErrMsg: fmt.Sprintf("%s only authenticates credential type BEARER", "native_realm"),
		},
		{
			name: "should_authenticate_apikey_successfully",
			args: args{
				cred: &auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
			},
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
				}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "native_realm",
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
				Role: auth.Role{
					Type:    auth.RoleProjectAdmin,
					Project: "paystack",
				},
				APIKey: &datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
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
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
				}, nil)

				uR.EXPECT().FindUserByID(gomock.Any(), "1234").Times(1).Return(&datastore.User{UID: "1234"}, nil)
			},
			want: &auth.AuthenticatedUser{
				AuthenticatedByRealm: "native_realm",
				Credential: auth.Credential{
					Type:   auth.CredentialTypeAPIKey,
					APIKey: "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash",
				},
				Role: auth.Role{
					Type:    auth.RoleProjectAdmin,
					Project: "paystack",
				},
				Metadata: &datastore.User{UID: "1234"},
				User:     &datastore.User{UID: "1234"},
				APIKey: &datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
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
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					Type:      datastore.PersonalKey,
					UserID:    "1234",
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
				}, nil)

				uR.EXPECT().FindUserByID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
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
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					DeletedAt: null.NewTime(time.Now(), true),
					ExpiresAt: null.Time{},
					CreatedAt: time.Time{},
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
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.APIKey{
					UID: "abcd",
					Role: auth.Role{
						Type:    auth.RoleProjectAdmin,
						Project: "paystack",
					},
					MaskID:    "DkwB9HnZxy4DqZMi",
					Hash:      "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM=",
					Salt:      "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g==",
					ExpiresAt: null.NewTime(time.Now().Add(time.Second*-10), true),
					DeletedAt: null.Time{},
					CreatedAt: time.Time{},
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
			nFn: func(aR *mocks.MockAPIKeyRepository, uR *mocks.MockUserRepository, pR *mocks.MockPortalLinkRepository) {
				aR.EXPECT().
					GetAPIKeyByMaskID(gomock.Any(), gomock.Any()).
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
			mockPortalLinkRepo := mocks.NewMockPortalLinkRepository(ctrl)

			nr := NewNativeRealm(mockApiKeyRepo, mockUserRepo, mockPortalLinkRepo)
			if tt.nFn != nil {
				tt.nFn(mockApiKeyRepo, mockUserRepo, mockPortalLinkRepo)
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

const (
	cachedTestKey      = "CO.DkwB9HnZxy4DqZMi.0JUxUfnQJ7NHqvD2ikHsHFx4Wd5nnlTMgsOfUs4eW8oU2G7dA75BWrHfFYYvrash"
	cachedTestMaskID   = "DkwB9HnZxy4DqZMi"
	cachedTestSalt     = "6y9yQZWqbE1AMHvfUewuYwasycmoe_zg5g=="
	cachedTestKeyHash  = "R4rtPIELUaJ9fx6suLreIpH3IaLzbxRcODy3a0Zm1qM="
	cachedTestNewSalt  = "Ck0FzSHHUmwPjGY3d0lqhwasycmoe_zg5g=="
	cachedTestWrongKey = "CO.DkwB9HnZxy4DqZMi.wrongsecretwrongsecretwrongsecretwrongsecretwrongsecretwrong"
)

// hashAPIKey mirrors how services/create_api_key.go persists a key, so fixtures
// are produced by the writer's derivation rather than by the realm's own.
func hashAPIKey(t *testing.T, key, salt string) string {
	t.Helper()

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	return base64.URLEncoding.EncodeToString(dk)
}

func newCachingTestRealm(t *testing.T) (*NativeRealm, *mocks.MockAPIKeyRepository) {
	t.Helper()

	ctrl := gomock.NewController(t)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)

	realm := NewNativeRealm(apiKeyRepo, mocks.NewMockUserRepository(ctrl), mocks.NewMockPortalLinkRepository(ctrl))
	return realm, apiKeyRepo
}

func liveAPIKey() *datastore.APIKey {
	return &datastore.APIKey{
		UID:    "abcd",
		Role:   auth.Role{Type: auth.RoleProjectAdmin, Project: "paystack"},
		MaskID: cachedTestMaskID,
		Hash:   cachedTestKeyHash,
		Salt:   cachedTestSalt,
	}
}

func authenticateAPIKey(realm *NativeRealm, key string) error {
	_, err := realm.Authenticate(context.Background(), &auth.Credential{
		Type:   auth.CredentialTypeAPIKey,
		APIKey: key,
	})
	return err
}

func TestNativeRealm_Authenticate_CachesOneEntryPerKey(t *testing.T) {
	realm, apiKeyRepo := newCachingTestRealm(t)
	apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Times(2).
		DoAndReturn(func(context.Context, string) (*datastore.APIKey, error) {
			return liveAPIKey(), nil
		})

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))
	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))

	require.Equal(t, 1, realm.derivedKeys.Len())
}

func TestNativeRealm_Authenticate_CacheHitStillHonoursRevocation(t *testing.T) {
	realm, apiKeyRepo := newCachingTestRealm(t)

	revoked := liveAPIKey()
	revoked.DeletedAt = null.NewTime(time.Now(), true)

	gomock.InOrder(
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(liveAPIKey(), nil),
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(revoked, nil),
	)

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))
	require.EqualError(t, authenticateAPIKey(realm, cachedTestKey), "api key has been revoked")
}

func TestNativeRealm_Authenticate_CacheHitStillHonoursExpiry(t *testing.T) {
	realm, apiKeyRepo := newCachingTestRealm(t)

	expired := liveAPIKey()
	expired.ExpiresAt = null.NewTime(time.Now().Add(-10*time.Second), true)

	gomock.InOrder(
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(liveAPIKey(), nil),
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(expired, nil),
	)

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))
	require.EqualError(t, authenticateAPIKey(realm, cachedTestKey), "api key has expired")
}

func TestNativeRealm_Authenticate_RejectsWrongKeyAfterCachedSuccess(t *testing.T) {
	realm, apiKeyRepo := newCachingTestRealm(t)
	apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Times(2).
		DoAndReturn(func(context.Context, string) (*datastore.APIKey, error) {
			return liveAPIKey(), nil
		})

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))
	require.EqualError(t, authenticateAPIKey(realm, cachedTestWrongKey), "invalid api key")

	// A mismatch is never admitted, so wrong keys cannot evict live entries.
	require.Equal(t, 1, realm.derivedKeys.Len())
}

func TestNativeRealm_Authenticate_CacheKeyCoversSalt(t *testing.T) {
	realm, apiKeyRepo := newCachingTestRealm(t)

	staleHash := liveAPIKey()
	staleHash.Salt = cachedTestNewSalt

	rotated := liveAPIKey()
	rotated.Salt = cachedTestNewSalt
	rotated.Hash = hashAPIKey(t, cachedTestKey, cachedTestNewSalt)

	gomock.InOrder(
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(liveAPIKey(), nil),
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(staleHash, nil),
		apiKeyRepo.EXPECT().GetAPIKeyByMaskID(gomock.Any(), cachedTestMaskID).Return(rotated, nil),
	)

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))

	// New salt, hash still derived under the old one. A cache keyed on the key
	// alone would answer this from the first derivation and wrongly admit it.
	require.EqualError(t, authenticateAPIKey(realm, cachedTestKey), "invalid api key")

	require.NoError(t, authenticateAPIKey(realm, cachedTestKey))
	require.Equal(t, 2, realm.derivedKeys.Len())
}

func BenchmarkNativeRealm_verifyAPIKey(b *testing.B) {
	want, err := base64.URLEncoding.DecodeString(cachedTestKeyHash)
	require.NoError(b, err)

	realm := NewNativeRealm(nil, nil, nil)

	b.Run("uncached", func(b *testing.B) {
		for b.Loop() {
			realm.derivedKeys.Purge()
			if !realm.verifyAPIKey(cachedTestKey, cachedTestSalt, want) {
				b.Fatal("expected key to verify")
			}
		}
	})

	b.Run("cached", func(b *testing.B) {
		realm.verifyAPIKey(cachedTestKey, cachedTestSalt, want)

		for b.Loop() {
			if !realm.verifyAPIKey(cachedTestKey, cachedTestSalt, want) {
				b.Fatal("expected key to verify")
			}
		}
	})
}
