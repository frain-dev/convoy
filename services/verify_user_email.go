package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type VerifyEmailService struct {
	UserRepo datastore.UserRepository
	Token    string
	Logger   log.Logger
}

func (u *VerifyEmailService) Run(ctx context.Context) error {
	user, err := u.UserRepo.FindUserByEmailVerificationToken(ctx, u.Token)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return &ServiceError{ErrMsg: "invalid password reset token"}
		}

		u.Logger.ErrorContext(ctx, "failed to find user by email verification token", "error", err)
		return &ServiceError{ErrMsg: "failed to find user", Err: err}
	}

	if time.Now().After(user.EmailVerificationExpiresAt) {
		return &ServiceError{ErrMsg: "email verification token has expired"}
	}

	user.EmailVerified = true
	err = u.UserRepo.UpdateUser(ctx, user)
	if err != nil {
		u.Logger.ErrorContext(ctx, "failed to update user", "error", err)
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	return nil
}
