package file

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
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
	Basic  []BasicAuth  `json:"basic"`
	APIKey []APIKeyAuth `json:"api_key"`
}

func (r *FileRealm) GetName() string {
	return auth.FileRealmName
}

func (r *FileRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
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
				AuthenticatedByRealm: r.GetName(),
				Credential:           *cred,
				Role:                 b.Role,
			}
			return authUser, nil
		}
		return nil, auth.ErrCredentialNotFound
	case auth.CredentialTypeAPIKey:
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
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type.String())
	}
}

// NewFileRealm constructs a new File Realm authenticator
func NewFileRealm(opts *config.FileRealmOption) (*FileRealm, error) {
	fr := &FileRealm{}
	for _, basicAuth := range opts.Basic {
		fr.Basic = append(fr.Basic, BasicAuth{
			Username: basicAuth.Username,
			Password: basicAuth.Password,
			Role: auth.Role{
				Type:    basicAuth.Role.Type,
				Project: basicAuth.Role.Project,
			},
		})
	}

	for _, basicAuth := range opts.APIKey {
		fr.APIKey = append(fr.APIKey, APIKeyAuth{
			APIKey: basicAuth.APIKey,
			Role: auth.Role{
				Type:    basicAuth.Role.Type,
				Project: basicAuth.Role.Project,
			},
		})
	}

	return fr, nil
}
