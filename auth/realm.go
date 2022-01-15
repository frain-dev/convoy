package auth

import "context"

type Realm interface {
	GetName() string
	Authenticate(ctx context.Context, cred *Credential) (*AuthenticatedUser, error)
}
