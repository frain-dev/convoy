package native

import (
	"context"
	"fmt"

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

	authUser := &auth.AuthenticatedUser{
		AuthenticatedByRealm: n.GetName(),
		Credential:           *cred,
		Role: auth.Role{
			Type:   "",
			Groups: []string{apiKey.Group},
		},
	}

	return authUser, nil
}

func (n *NativeRealm) GetName() string {
	return "native_realm"
}
