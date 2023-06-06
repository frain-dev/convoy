package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
)

type ResendEmailVerificationTokenService struct {
	UserRepo datastore.UserRepository
	Queue    queue.Queuer

	BaseURL string
	User    *datastore.User
}

func (u *ResendEmailVerificationTokenService) Run(ctx context.Context) error {
	if u.User.EmailVerified {
		return &ServiceError{ErrMsg: "user email already verified"}
	}

	if u.User.EmailVerificationExpiresAt.After(time.Now()) {
		return &ServiceError{ErrMsg: "old verification token is still valid"}
	}

	u.User.EmailVerificationExpiresAt = time.Now().Add(time.Hour * 2)
	u.User.EmailVerificationToken = ulid.Make().String()

	err := u.UserRepo.UpdateUser(ctx, u.User)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update user")
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	err = sendUserVerificationEmail(ctx, u.BaseURL, u.User, u.Queue)
	if err != nil {
		return &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
	}

	return nil
}
