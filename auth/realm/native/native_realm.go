package native

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/crypto/pbkdf2"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

const (
	// Fixed by the keys already in the database, not tunable: these must stay in
	// step with the derivation the key-creation services persist, so raising
	// either value would reject every existing key.
	pbkdf2Iterations = 4096
	pbkdf2KeyLength  = 32

	// Sized for the number of distinct API keys an instance authenticates with,
	// not for request volume, since one key serves every request it makes.
	derivedKeyCacheSize = 10_000
	derivedKeyCacheTTL  = 30 * time.Minute
)

type NativeRealm struct {
	apiKeyRepo        datastore.APIKeyRepository
	userRepo          datastore.UserRepository
	portalLinkService datastore.PortalLinkRepository

	// PBKDF2 over 4096 HMAC-SHA256 rounds was 30% of data plane agent CPU under
	// load, because the derivation ran per request rather than per key. What is
	// cached is only the output of a pure function of (key, salt), so it cannot
	// carry stale authorization: expiry, revocation and role are still read from
	// the API key row on every request below.
	derivedKeys *expirable.LRU[string, []byte]
}

func NewNativeRealm(apiKeyRepo datastore.APIKeyRepository,
	userRepo datastore.UserRepository,
	portalLinkService datastore.PortalLinkRepository) *NativeRealm {
	return &NativeRealm{
		apiKeyRepo:        apiKeyRepo,
		userRepo:          userRepo,
		portalLinkService: portalLinkService,
		derivedKeys: expirable.NewLRU[string, []byte](
			derivedKeyCacheSize, nil, derivedKeyCacheTTL),
	}
}

// verifyAPIKey reports whether key derives to want under salt, caching the
// derivation so the 4096-round PBKDF2 cost is paid per key rather than per
// request.
//
// Only a match is admitted to the cache. Caching a mismatch would let a flood of
// wrong keys evict live entries, and leaving the full derivation cost on the
// failing path keeps brute force expensive.
func (n *NativeRealm) verifyAPIKey(key, salt string, want []byte) bool {
	// The cache key covers the salt as well as the key, so a rotated salt is not
	// answered with the derivation from the previous one. want is compared on
	// every call, including cache hits, so a re-hashed key still fails here.
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte{0})
	h.Write([]byte(key))
	cacheKey := string(h.Sum(nil))

	if dk, ok := n.derivedKeys.Get(cacheKey); ok {
		return subtle.ConstantTimeCompare(dk, want) == 1
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), pbkdf2Iterations, pbkdf2KeyLength, sha256.New)
	if subtle.ConstantTimeCompare(dk, want) != 1 {
		return false
	}

	n.derivedKeys.Add(cacheKey, dk)

	return true
}

func (n *NativeRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeAPIKey {
		return nil, fmt.Errorf("%s only authenticates credential type %s", n.GetName(), auth.CredentialTypeAPIKey.String())
	}

	keySplit := strings.Split(cred.APIKey, ".")

	if len(keySplit) != 3 {
		return nil, errors.New("invalid api key format")
	}

	maskID := keySplit[1]
	apiKey, err := n.apiKeyRepo.GetAPIKeyByMaskID(ctx, maskID)
	if err != nil {
		return nil, fmt.Errorf("failed to hash api key: %v", err)
	}

	decodedKey, err := base64.URLEncoding.DecodeString(apiKey.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode string: %v", err)
	}

	// compute hash & compare.
	if !n.verifyAPIKey(cred.APIKey, apiKey.Salt, decodedKey) {
		// Not Match.
		return nil, errors.New("invalid api key")
	}

	// if the current time is after the specified expiry date then the key has expired
	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt.ValueOrZero()) {
		return nil, errors.New("api key has expired")
	}

	if !apiKey.DeletedAt.IsZero() {
		return nil, errors.New("api key has been revoked")
	}

	authUser := &auth.AuthenticatedUser{
		AuthenticatedByRealm: n.GetName(),
		Credential:           *cred,
		Role:                 apiKey.Role,
		APIKey:               apiKey,
	}

	if apiKey.Type == datastore.PersonalKey {
		user, innerErr := n.userRepo.FindUserByID(ctx, apiKey.UserID)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to fetch user: %v", innerErr)
		}

		authUser.Metadata = user
		authUser.User = user
	}

	return authUser, nil
}

func (n *NativeRealm) GetName() string {
	return auth.NativeRealmName
}
