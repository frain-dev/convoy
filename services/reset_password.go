package services

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

type ResetPasswordService struct {
	UserRepo datastore.UserRepository

	Token string
	Data  *models.ResetPassword
}

func (u *ResetPasswordService) Run(ctx context.Context) (*datastore.User, error) {
	user, err := u.UserRepo.FindUserByToken(ctx, u.Token)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return nil, &ServiceError{ErrMsg: "invalid password reset token"}
		}

		log.FromContext(ctx).WithError(err).Error("failed to find user by reset password token")
		return nil, &ServiceError{ErrMsg: "failed to find user by reset password token", Err: err}
	}

	if time.Now().After(user.ResetPasswordExpiresAt) {
		return nil, &ServiceError{ErrMsg: "password reset token has expired"}
	}

	if u.Data.Password != u.Data.PasswordConfirmation {
		return nil, &ServiceError{ErrMsg: "password confirmation doesn't match password"}
	}

	p := datastore.Password{Plaintext: u.Data.Password}
	err = p.GenerateHash()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate hash")
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	user.Password = string(p.Hash)
	err = u.UserRepo.UpdateUser(ctx, user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("an error occurred while updating user")
		return nil, &ServiceError{ErrMsg: "an error occurred while updating user", Err: err}
	}

	return user, nil
}
