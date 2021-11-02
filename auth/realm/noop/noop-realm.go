package noop

import "github.com/frain-dev/convoy/auth"

type NoopRealm struct{}

func (n NoopRealm) Name() string {
	return "noop_realm"
}

var authUser = &auth.AuthenticatedUser{
	Credential: auth.Credential{
		Type:     "",
		Username: "",
		Password: "",
		APIKey:   "",
	},
	Roles: []auth.Role{auth.RoleSuperUser},
}

func (n NoopRealm) Authenticate(cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	return nil, nil
}

func NewNoopRealm() auth.Realm {
	return &NoopRealm{}
}
