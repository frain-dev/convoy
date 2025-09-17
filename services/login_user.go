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

func (u *LoginUserService) isPrimaryInstanceAdmin(ctx context.Context, userID string) (bool, error) {
	if u.Licenser.MultiPlayerMode() {
		// If licensed, all users can access
		return true, nil
	}

	count, err := u.OrgMemberRepo.CountInstanceAdminUsers(ctx)
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}

	isFirst, err := u.OrgMemberRepo.IsFirstInstanceAdmin(ctx, userID)
	if err != nil {
		return false, err
	}

	return isFirst, nil
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

	// Check if user can access based on license status and get instance admin count
	canAccess, err := u.isPrimaryInstanceAdmin(ctx, user.UID)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !canAccess {
		return nil, nil, &ServiceError{
			Code:   ErrCodeLicenseExpired,
			ErrMsg: "License expired. Only the primary instance administrator can access the system"}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	return user, &token, nil
}
