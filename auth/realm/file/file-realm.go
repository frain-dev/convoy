package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
	Group string    `json:"group"` // TODO(daniel,subomi): we should remove this, as it will cause major complexities in the future
}

type FileRealm struct {
	Name   string       `json:"name"`
	Basic  []BasicAuth  `json:"basic"`
	APIKey []APIKeyAuth `json:"api_key"`
}

func (r *FileRealm) GetName() string {
	return r.Name
}

func (r *FileRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	switch cred.Type {
	case auth.CredentialTypeBasic:
		for _, b := range r.Basic {
			if cred.Username != b.Username {
				continue
			}

			if cred.Password != b.Password {
				continue
			}

			authUser := &auth.AuthenticatedUser{
				AuthenticatedByRealm: r.Name,
				Credential:           *cred,
				Roles:                []auth.Role{b.Role.Type},
			}
			return authUser, nil
		}
		return nil, ErrCredentialNotFound
	case auth.CredentialTypeAPIKey:
		for _, b := range r.APIKey {
			if cred.APIKey != b.APIKey {
				continue
			}

			authUser := &auth.AuthenticatedUser{
				AuthenticatedByRealm: r.Name,
				Credential:           *cred,
				Roles:                []auth.Role{b.Role.Type},
			}
			return authUser, nil
		}
		return nil, ErrCredentialNotFound
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type.String())
	}
}

// NewFileRealm constructs a new File Realm authenticator
func NewFileRealm(path string) (*FileRealm, error) {
	r := &FileRealm{}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, r)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal file %s into new file realm: %v", path, err)
	}

	return r, nil
}
