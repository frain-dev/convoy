package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
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
			return nil, nil, &ServiceError{Code: ErrCodeAuthInvalid, ErrMsg: "invalid username or password", Err: err}
		}

		return nil, nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: err.Error(), Err: err}
	}

	p := datastore.Password{Plaintext: u.Data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: err.Error(), Err: err}
	}
	if !match {
		return nil, nil, &ServiceError{Code: ErrCodeAuthInvalid, ErrMsg: "invalid username or password"}
	}

	canAccess, err := PrimaryInstanceAccess(ctx, user.UID, u.UserRepo, u.OrgMemberRepo, u.Licenser)
	if err != nil {
		return nil, nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: err.Error(), Err: err}
	}

	if !canAccess {
		return nil, nil, &ServiceError{
			Code: ErrCodeLicenseAccessDenied,
			ErrMsg: "This instance does not allow your account to sign in under the current license. " +
				"Sign in as an instance administrator, enable multi-user licensing, or contact your administrator.",
		}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		return nil, nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: err.Error(), Err: err}
	}

	return user, &token, nil
}
