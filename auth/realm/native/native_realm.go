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
	apiKeyRepo     datastore.APIKeyRepository
	userRepo       datastore.UserRepository
	portalLinkRepo datastore.PortalLinkRepository
}

func NewNativeRealm(apiKeyRepo datastore.APIKeyRepository,
	userRepo datastore.UserRepository,
	portalLinkRepo datastore.PortalLinkRepository) *NativeRealm {
	return &NativeRealm{apiKeyRepo: apiKeyRepo, userRepo: userRepo, portalLinkRepo: portalLinkRepo}
}

func (n *NativeRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type == auth.CredentialTypeToken {
		pLink, err := n.portalLinkRepo.FindPortalLinkByToken(ctx, cred.Token)
		if err != nil {
			return nil, errors.New("invalid portal link token")
		}

		return &auth.AuthenticatedUser{
			AuthenticatedByRealm: n.GetName(),
			Credential:           *cred,
			PortalLink:           pLink,
		}, nil
	}

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
	return auth.NativeRealmName
}
