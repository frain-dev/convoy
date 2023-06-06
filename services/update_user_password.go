package services

import (
	"context"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type UpdatePasswordService struct {
	UserRepo datastore.UserRepository

	Data *models.UpdatePassword
	User *datastore.User
}

func (u *UpdatePasswordService) Run(ctx context.Context) (*datastore.User, error) {
	p := datastore.Password{Plaintext: u.Data.CurrentPassword, Hash: []byte(u.User.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !match {
		return nil, &ServiceError{ErrMsg: "current password is invalid"}
	}

	if u.Data.Password != u.Data.PasswordConfirmation {
		return nil, &ServiceError{ErrMsg: "password confirmation doesn't match password"}
	}

	p.Plaintext = u.Data.Password
	err = p.GenerateHash()

	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate hash")
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	u.User.Password = string(p.Hash)
	err = u.UserRepo.UpdateUser(ctx, u.User)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("an error occurred while updating user")
		return nil, &ServiceError{ErrMsg: "an error occurred while updating user", Err: err}
	}

	return u.User, nil
}
