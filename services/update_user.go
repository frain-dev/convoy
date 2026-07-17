package services

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type UpdateUserService struct {
	UserRepo datastore.UserRepository
	Queue    queue.Queuer

	BaseURL string
	Data    *models.UpdateUser
	User    *datastore.User
	Logger  log.Logger
}

func (u *UpdateUserService) Run(ctx context.Context) (*datastore.User, error) {
	if !u.User.EmailVerified {
		return nil, &ServiceError{ErrMsg: "email has not been verified"}
	}

	// Verification is bound to the address it was performed for. Changing the
	// email un-verifies the account (fail closed for gates that check
	// EmailVerified, e.g. cloud trial start) and issues a fresh token so the
	// new address must be verified.
	emailChanged := !strings.EqualFold(strings.TrimSpace(u.Data.Email), strings.TrimSpace(u.User.Email))

	u.User.FirstName = u.Data.FirstName
	u.User.LastName = u.Data.LastName
	u.User.Email = u.Data.Email

	if emailChanged {
		u.User.EmailVerified = false
		u.User.EmailVerificationToken = ulid.Make().String()
		u.User.EmailVerificationExpiresAt = time.Now().Add(time.Hour * 2)
	}

	err := u.UserRepo.UpdateUser(ctx, u.User)
	if err != nil {
		u.Logger.ErrorContext(ctx, "failed to update user", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	if emailChanged {
		err = sendUserVerificationEmail(ctx, u.BaseURL, u.User, u.Queue, u.Logger)
		if err != nil {
			return nil, &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
		}
	}

	return u.User, nil
}
