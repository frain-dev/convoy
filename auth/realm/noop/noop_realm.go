package noop

import (
	"context"

	"github.com/frain-dev/convoy/auth"
)

type NoopRealm struct{}

func (n NoopRealm) GetName() string {
	return auth.NoopRealmName
}

var authUser = &auth.AuthenticatedUser{
	Credential: auth.Credential{
		Type:     auth.CredentialTypeBasic,
		Username: "default",
		Password: "default",
		APIKey:   "",
	},
	Role: auth.Role{Type: auth.RoleSuperUser, Project: ""},
}

func (n NoopRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	return authUser, nil
}

func NewNoopRealm() NoopRealm {
	return NoopRealm{}
}
