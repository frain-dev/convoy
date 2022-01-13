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
}

func NewNativeRealm(apiKeyRepo datastore.APIKeyRepository) *NativeRealm {
	return &NativeRealm{apiKeyRepo: apiKeyRepo}
}

func (n *NativeRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeAPIKey {
		return nil, fmt.Errorf("%s only authenticates credential type %s", n.GetName(), auth.CredentialTypeAPIKey.String())
	}

	key := cred.APIKey
	maskID := strings.Split(key, ".")[1]

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

	if bytes.Compare(dk, decodedKey) != 0 {
		// Not Match.
		return nil, errors.New("invalid api key")
	}

	if apiKey.DeletedAt != 0 {
		return nil, errors.New("api key has been revoked")
	}

	// if the current time is after the specified expiry date then the key has expired
	if apiKey.ExpiresAt != 0 && time.Now().After(apiKey.ExpiresAt.Time()) {
		return nil, errors.New("api key has expired")
	}

	authUser := &auth.AuthenticatedUser{
		AuthenticatedByRealm: n.GetName(),
		Credential:           *cred,
		Role:                 apiKey.Role,
	}

	return authUser, nil
}

func (n *NativeRealm) GetName() string {
	return "native_realm"
}
