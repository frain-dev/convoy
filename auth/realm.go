package auth

type Realm interface {
	GetName() string
	Authenticate(cred *Credential) (*AuthenticatedUser, error)
}
