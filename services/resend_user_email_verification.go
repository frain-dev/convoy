package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
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
		slog.ErrorContext(ctx, "failed to update user", "error", err)
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	err = sendUserVerificationEmail(ctx, u.BaseURL, u.User, u.Queue)
	if err != nil {
		return &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
	}

	return nil
}
