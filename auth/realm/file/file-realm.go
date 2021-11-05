package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/frain-dev/convoy/auth"
	log "github.com/sirupsen/logrus"
)

var (
	ErrCredentialNotFound = errors.New("credential not found")
)

type BasicAuth struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

type APIKeyAuth struct {
	APIKey string    `json:"api_key"`
	Role   auth.Role `json:"role"`
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
				Role:                 b.Role,
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
				Role:                 b.Role,
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
	fr := &FileRealm{}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, fr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal file %s into new file realm: %v", path, err)
	}

	if fr.Basic == nil && fr.APIKey == nil {
		log.Warnf("no authentication data supplied in file realm '%s", fr.Name)
	}

	return fr, nil
}
