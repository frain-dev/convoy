package api_key

import (
	"fmt"

	"github.com/frain-dev/convoy/auth"
)

type APIKeyRealm struct {
	APIKey []auth.APIKeyAuth `json:"api_key"`
}

func (r *APIKeyRealm) GetName() string {
	return "api_key_realm"
}

func (r *APIKeyRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeAPIKey {
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type.String())
	}

	for _, b := range r.APIKey {
		if cred.APIKey != b.APIKey {
			continue
		}

		authUser := &auth.AuthenticatedUser{
			AuthenticatedByRealm: r.GetName(),
			Credential:           *cred,
			Role:                 b.Role,
		}
		return authUser, nil
	}
	return nil, auth.ErrCredentialNotFound

}

// NewAPIKeyRealm constructs a new APIKeyRealm Realm authenticator
func NewAPIKeyRealm(apiKeyList []auth.APIKeyAuth) (*APIKeyRealm, error) {
	br := &APIKeyRealm{APIKey: apiKeyList}

	if len(br.APIKey) == 0 {
		return nil, fmt.Errorf("no authentication data supplied for '%s'", br.GetName())
	}

	return br, nil
}
