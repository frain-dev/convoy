package auth

import "errors"

var ErrCredentialNotFound = errors.New("credential not found")

type AuthenticatedUser struct {
	AuthenticatedByRealm string      `json:"-"` // Name of realm that authenticated this user
	Credential           Credential  `json:"credential"`
	Role                 Role        `json:"role"`
	Metadata             interface{} `json:"-"` // Additional data set by the realm that authenticated the user, see the jwt realm for an example

	// TODO(subomi): This are set to interfaces temporarily to work around import cycles.
	User   interface{} `json:"user"`
	APIKey interface{} `json:"api_key"`
}

type Credential struct {
	Type     CredentialType `json:"type"`
	Username string         `json:"username"`
	Password string         `json:"password"`
	APIKey   string         `json:"api_key"`
	Token    string         `json:"token"`
}

func (c *Credential) String() string {
	return c.Username
}

type CredentialType string

const (
	CredentialTypeBasic  = CredentialType("BASIC")
	CredentialTypeAPIKey = CredentialType("BEARER")
	CredentialTypeJWT    = CredentialType("JWT")
)

const (
	NativeRealmName = "native_realm"
	JWTRealmName    = "jwt"
	FileRealmName   = "file_realm"
	NoopRealmName   = "noop_realm"
)

func (c CredentialType) String() string {
	return string(c)
}
