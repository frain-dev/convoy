package noop

import "github.com/frain-dev/convoy/auth"

type NoopRealm struct{}

func (n NoopRealm) GetName() string {
	return "noop_realm"
}

var authUser = &auth.AuthenticatedUser{
	Credential: auth.Credential{
		Type:     auth.CredentialTypeBasic,
		Username: "default",
		Password: "default",
		APIKey:   "",
	},
	Roles: []auth.Role{auth.RoleSuperUser},
}

func (n NoopRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	return authUser, nil
}

func NewNoopRealm() *NoopRealm {
	return &NoopRealm{}
}
