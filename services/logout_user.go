package services

import (
	"context"
	"log/slog"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
)

type LogoutUserService struct {
	JWT      *jwt.Jwt
	UserRepo datastore.UserRepository
	Token    string
}

func (u *LogoutUserService) Run(ctx context.Context) error {
	verified, err := u.JWT.ValidateAccessToken(u.Token)
	if err != nil {
		slog.ErrorContext(ctx, "failed to validate token", "error", err)
		return &ServiceError{ErrMsg: "failed to validate token", Err: err}
	}

	err = u.JWT.BlacklistToken(verified, u.Token)
	if err != nil {
		slog.ErrorContext(ctx, "failed to blacklist token", "error", err)
		return &ServiceError{ErrMsg: "failed to blacklist token", Err: err}
	}

	return nil
}
