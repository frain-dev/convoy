package basic

import (
	"fmt"

	"github.com/frain-dev/convoy/auth"
)

type BasicRealm struct {
	Basic []auth.BasicAuth `json:"basic"`
}

func (r *BasicRealm) GetName() string {
	return "basic_realm"
}

func (r *BasicRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeBasic {
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type.String())
	}

	for _, b := range r.Basic {
		if cred.Username != b.Username {
			continue
		}

		if cred.Password != b.Password {
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

// NewBasicRealm constructs a new Basic Realm authenticator
func NewBasicRealm(basicList []auth.BasicAuth) (*BasicRealm, error) {
	br := &BasicRealm{Basic: basicList}
	if len(br.Basic) == 0 {
		return nil, fmt.Errorf("no authentication data supplied for '%s'", br.GetName())
	}

	return br, nil
}
