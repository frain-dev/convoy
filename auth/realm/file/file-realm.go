package file

import (
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/auth"
)

var (
	ErrCredentialNotFound = errors.New("credential not found")
)

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type APIKeyAuth struct {
	APIKey string `json:"api_key"`
	Role   Role   `json:"role"`
}

type Role struct {
	Type  auth.Role `json:"type"`
	Group string    `json:"group"` // we should remove this, as it will cause major complexities in the future
}

type FileRealm struct {
	name   string
	Basic  []BasicAuth  `json:"basic"`
	APIKey []APIKeyAuth `json:"api_key"`
}

func (f *FileRealm) Name() string {
	return f.name
}

func (f *FileRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	switch cred.Type {
	case auth.CredentialTypeBasic:
		for _, b := range f.Basic {
			if cred.Username != b.Username {
				continue
			}

			if cred.Password != b.Password {
				continue
			}

			authUser := &auth.AuthenticatedUser{
				Credential: *cred,
				Roles:      []auth.Role{b.Role.Type},
			}
			return authUser, nil
		}
		return nil, ErrCredentialNotFound
	case auth.CredentialTypeAPIKey:
		for _, b := range f.APIKey {
			if cred.APIKey != b.APIKey {
				continue
			}

			authUser := &auth.AuthenticatedUser{
				Credential: *cred,
				Roles:      []auth.Role{b.Role.Type},
			}
			return authUser, nil
		}
		return nil, ErrCredentialNotFound
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type.String())

	}
}

func NewFileRealm() auth.Realm {
	return &FileRealm{}
}
