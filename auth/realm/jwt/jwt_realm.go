package jwt

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type JwtRealm struct {
	userRepo datastore.UserRepository
	jwt      *Jwt
}

func NewJwtRealm(userRepo datastore.UserRepository, opts *config.JwtRealmOptions, cache cache.Cache) *JwtRealm {
	return &JwtRealm{userRepo: userRepo, jwt: NewJwt(opts, cache)}
}

func (j *JwtRealm) Authenticate(ctx context.Context, cred *auth.Credential) (*auth.AuthenticatedUser, error) {
	if cred.Type != auth.CredentialTypeJWT {
		return nil, fmt.Errorf("%s only authenticates credential type %s", j.GetName(), auth.CredentialTypeJWT.String())
	}

	verified, err := j.jwt.ValidateAccessToken(cred.Token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := j.userRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	authUser := &auth.AuthenticatedUser{
		AuthenticatedByRealm: j.GetName(),
		Credential:           *cred,
		Role:                 auth.Role{},
		Metadata:             user,
		User:                 user,
	}

	return authUser, nil
}

func (j *JwtRealm) GetName() string {
	return "jwt"
}
