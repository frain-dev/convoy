package auth

type Realm interface {
	GetName() string
	Authenticate(cred *Credential) (*AuthenticatedUser, error)
}

type RealmType string

const (
	RealmTypeFile = RealmType("file_realm")
	//RealmTypeVault = RealmType("vault_realm")
)
