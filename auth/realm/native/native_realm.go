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
			// cred.Token should be the owner id at this point
			pLinks, innerErr := n.portalLinkRepo.FindPortalLinksByOwnerID(ctx, cred.Token)
			if innerErr != nil {
				return nil, innerErr
			}

			if len(pLinks) == 0 {
				return nil, err
			}

			pLink = &pLinks[0]
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
		if !errors.Is(err, datastore.ErrAPIKeyNotFound) {
			return nil, fmt.Errorf("failed to find api key: %v", err)
		}

		// check if the api key is a portal link auth token
		pLink, innerErr := n.portalLinkRepo.FindPortalLinkByMaskId(ctx, maskID)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to find portal link: %v", innerErr)
		}

		// if the portal link is found, use the token hash and salt
		decodedKey, innerErr := base64.URLEncoding.DecodeString(pLink.TokenHash)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to decode string: %v", innerErr)
		}

		// compute hash & compare.
		dk := pbkdf2.Key([]byte(cred.APIKey), []byte(pLink.TokenSalt), 4096, 32, sha256.New)

		if !bytes.Equal(dk, decodedKey) {
			// Not Match.
			return nil, errors.New("invalid portal link auth token")
		}

		// if the current time is after the specified expiry date then the key has expired
		if !pLink.TokenExpiresAt.IsZero() && time.Now().After(pLink.TokenExpiresAt.ValueOrZero()) {
			return nil, errors.New("portal link auth token has expired")
		}

		if !pLink.DeletedAt.IsZero() {
			return nil, errors.New("portal link auth token has been revoked")
		}

		return &auth.AuthenticatedUser{
			AuthenticatedByRealm: n.GetName(),
			Credential:           *cred,
			PortalLink:           pLink,
		}, nil
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
