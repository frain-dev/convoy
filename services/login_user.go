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

func NewLoginUserService(
	userRepo datastore.UserRepository,
	orgMemberRepo datastore.OrganisationMemberRepository,
	cache cache.Cache,
	jwt *jwt.Jwt,
	data *models.LoginUser,
	licenser license.Licenser,
) *LoginUserService {
	return &LoginUserService{
		UserRepo:      userRepo,
		OrgMemberRepo: orgMemberRepo,
		Cache:         cache,
		JWT:           jwt,
		Data:          data,
		Licenser:      licenser,
	}
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
	canAccess, err := IsPrimaryInstanceAdmin(ctx, u.Licenser, u.OrgMemberRepo, u.UserRepo, user.UID)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !canAccess {
		return nil, nil, &ServiceError{
			Code:   ErrCodeLicenseExpired,
			ErrMsg: "License expired. Only the first organization administrator can access the system"}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	return user, &token, nil
}
