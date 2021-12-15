package native

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/util"
)

type NativeRealm struct {
	apiKeyRepo convoy.APIKeyRepo
}

func NewNativeRealm(apiKeyRepo convoy.APIKeyRepo) *NativeRealm {
	return &NativeRealm{apiKeyRepo: apiKeyRepo}
}

func (n *NativeRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeAPIKey {
		return nil, fmt.Errorf("%s only authenticates credential type %s", n.GetName(), auth.CredentialTypeAPIKey.String())
	}

	hash, err := util.ComputeSHA256(cred.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to hash api key: %v", err)
	}

	apiKey, err := n.apiKeyRepo.FindAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to hash api key: %v", err)
	}

	if apiKey.Revoked {
		return nil, errors.New("api key has been revoked")
	}

	// if the current time id after the specified expiry date then the key has expired
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
