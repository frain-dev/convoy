package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
)

type LoginUserService struct {
	UserRepo      datastore.UserRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Cache         cache.Cache
	JWT           *jwt.Jwt
	Data          *models.LoginUser
	Licenser      license.Licenser
}

func (u *LoginUserService) Run(ctx context.Context) (*datastore.User, *jwt.Token, error) {
	user, err := u.UserRepo.FindUserByEmail(ctx, u.Data.Username)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return nil, nil, &ServiceError{ErrMsg: "invalid username or password", Err: err}
		}

		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	p := datastore.Password{Plaintext: u.Data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}
	if !match {
		return nil, nil, &ServiceError{ErrMsg: "invalid username or password"}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !u.Licenser.MultiPlayerMode() {
		hasAccess, err := u.OrgMemberRepo.HasInstanceAdminAccess(ctx, user.UID)
		if err != nil {
			return nil, nil, &ServiceError{ErrMsg: err.Error()}
		}

		if !hasAccess {
			return nil, nil, &ServiceError{ErrMsg: "License expired. Only instance admins can access the system"}
		}
	}

	return user, &token, nil
}
