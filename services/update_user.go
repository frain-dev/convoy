package services

import (
	"context"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type UpdateUserService struct {
	UserRepo datastore.UserRepository

	Data *models.UpdateUser
	User *datastore.User
}

func (u *UpdateUserService) Run(ctx context.Context) (*datastore.User, error) {
	if !u.User.EmailVerified {
		return nil, &ServiceError{ErrMsg: "email has not been verified"}
	}

	u.User.FirstName = u.Data.FirstName
	u.User.LastName = u.Data.LastName
	u.User.Email = u.Data.Email

	err := u.UserRepo.UpdateUser(ctx, u.User)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update user")
		return nil, &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	return u.User, nil
}
