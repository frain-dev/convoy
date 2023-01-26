package native

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"golang.org/x/crypto/pbkdf2"
)

type NativeRealm struct {
	apiKeyRepo datastore.APIKeyRepository
	userRepo   datastore.UserRepository
}

func NewNativeRealm(apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository) *NativeRealm {
	return &NativeRealm{apiKeyRepo: apiKeyRepo, userRepo: userRepo}
}

func (n *NativeRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeAPIKey {
		return nil, fmt.Errorf("%s only authenticates credential type %s", n.GetName(), auth.CredentialTypeAPIKey.String())
	}

	key := cred.APIKey
	keySplit := strings.Split(key, ".")

	if len(keySplit) != 3 {
		return nil, errors.New("invalid api key format")
	}

	maskID := keySplit[1]
	apiKey, err := n.apiKeyRepo.FindAPIKeyByMaskID(ctx, maskID)
	if err != nil {
		return nil, fmt.Errorf("failed to hash api key: %v", err)
	}

	decodedKey, err := base64.URLEncoding.DecodeString(apiKey.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode string: %v", err)
	}

	// compute hash & compare.
	dk := pbkdf2.Key([]byte(cred.APIKey), []byte(apiKey.Salt), 4096, 32, sha256.New)

	if !bytes.Equal(dk, decodedKey) {
		// Not Match.
		return nil, errors.New("invalid api key")
	}

	// if the current time is after the specified expiry date then the key has expired
	if apiKey.ExpiresAt != 0 && time.Now().After(apiKey.ExpiresAt.Time()) {
		return nil, errors.New("api key has expired")
	}

	if apiKey.DeletedAt != nil {
		return nil, errors.New("api key has been revoked")
	}

	authUser := &auth.AuthenticatedUser{
		AuthenticatedByRealm: n.GetName(),
		Credential:           *cred,
		Role: auth.Role{
			Type:     apiKey.RoleType,
			Project:  apiKey.RoleProject,
			Endpoint: apiKey.RoleEndpoint,
		},
		APIKey: apiKey,
	}

	if apiKey.Type == datastore.PersonalKey {
		user, err := n.userRepo.FindUserByID(ctx, apiKey.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user: %v", err)
		}

		authUser.Metadata = user
		authUser.User = user
	}

	return authUser, nil
}

func (n *NativeRealm) GetName() string {
	return "native_realm"
}
