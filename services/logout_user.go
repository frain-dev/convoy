package services

import (
	"context"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type LogoutUserService struct {
	JWT      *jwt.Jwt
	UserRepo datastore.UserRepository
	Token    string
}

func (u *LogoutUserService) Run(ctx context.Context) error {
	verified, err := u.JWT.ValidateAccessToken(u.Token)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate token")
		return &ServiceError{ErrMsg: "failed to validate token", Err: err}
	}

	err = u.JWT.BlacklistToken(verified, u.Token)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to blacklist token")
		return &ServiceError{ErrMsg: "failed to blacklist token", Err: err}
	}

	return nil
}
