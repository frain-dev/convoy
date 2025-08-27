package portal

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
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/xdg-go/pbkdf2"
)

type PortalRealm struct {
	portalLinkRepo datastore.PortalLinkRepository
	logger         log.StdLogger
}

func (p *PortalRealm) GetName() string {
	return auth.PortalRealmName
}

func NewPortalRealm(portalLinkRepo datastore.PortalLinkRepository, logger log.StdLogger) *PortalRealm {
	return &PortalRealm{
		portalLinkRepo: portalLinkRepo,
		logger:         logger,
	}
}

func (p *PortalRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	// this is where we'll switch portal auth types
	if len(cred.Token) > 0 { // this is the legacy static token type
		pLink, err := p.portalLinkRepo.FindPortalLinkByToken(ctx, cred.Token)
		if err != nil {
			// cred.Token should be the owner id at this point
			pLinks, innerErr := p.portalLinkRepo.FindPortalLinksByOwnerID(ctx, cred.Token)
			if innerErr != nil {
				return nil, innerErr
			}

			if len(pLinks) == 0 {
				return nil, err
			}

			pLink = &pLinks[0]
		}

		if pLink.AuthType == datastore.PortalAuthTypeStaticToken {
			return &auth.AuthenticatedUser{
				AuthenticatedByRealm: p.GetName(),
				Credential:           *cred,
				PortalLink:           pLink,
			}, nil
		}
	}

	keySplit := strings.Split(cred.APIKey, ".")

	if len(keySplit) != 3 {
		return nil, errors.New("invalid api key format")
	}

	maskID := keySplit[1]

	// check if the api key is a portal link auth token
	pLink, innerErr := p.portalLinkRepo.FindPortalLinkByMaskId(ctx, maskID)
	if innerErr != nil {
		p.logger.Warnf("failed to find portal link: %v", innerErr)
		return nil, fmt.Errorf("failed to find portal link: %v", innerErr)
	}

	if pLink.AuthType != datastore.PortalAuthTypeRefreshToken {
		return nil, errors.New("invalid portal link auth token type")
	}

	// if the portal link is found, use the token hash and salt
	decodedKey, innerErr := base64.URLEncoding.DecodeString(pLink.TokenHash)
	if innerErr != nil {
		p.logger.Warnf("failed to decode string: %v", innerErr)
		return nil, fmt.Errorf("failed to decode string: %v", innerErr)
	}

	// compute hash & compare.
	dk := pbkdf2.Key([]byte(cred.APIKey), []byte(pLink.TokenSalt), 4096, 32, sha256.New)
	if !bytes.Equal(dk, decodedKey) {
		return nil, errors.New("invalid portal link auth token")
	}

	// if the current time is after the specified expiry date, then the key has expired
	if !pLink.TokenExpiresAt.IsZero() && time.Now().After(pLink.TokenExpiresAt.ValueOrZero()) {
		return nil, errors.New("portal link auth token has expired")
	}

	if !pLink.DeletedAt.IsZero() {
		return nil, errors.New("portal link auth token has been revoked")
	}

	return &auth.AuthenticatedUser{
		AuthenticatedByRealm: p.GetName(),
		Credential:           *cred,
		PortalLink:           pLink,
	}, nil
}
