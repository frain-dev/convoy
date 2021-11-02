package auth

type Realm interface {
	Name() string
	Authenticate(cred *Credential) (*AuthenticatedUser, error)
}

type AuthenticatedUser struct {
	Credential Credential
	Roles      []Role
}

type Credential struct {
	Type     CredentialType `json:"type"`
	Username string         `json:"username"`
	Password string         `json:"password"`
	APIKey   string         `json:"api_key"`
}

type CredentialType string

const (
	CredentialTypeBasic  = CredentialType("BASIC")
	CredentialTypeAPIKey = CredentialType("API_KEY")
)

func (c CredentialType) String() string {
	return string(c)
}

type Role string

const (
	RoleSuperUser = Role("super_user")
	RoleUIAdmin   = Role("ui_admin")
	RoleAdmin     = Role("admin")
	RoleAPI       = Role("api")
)
