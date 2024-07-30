package services

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
)

type RefreshTokenService struct {
	UserRepo datastore.UserRepository
	JWT      *jwt.Jwt

	Data *models.Token
}

func (u *RefreshTokenService) Run(ctx context.Context) (*jwt.Token, error) {
	isValid, err := u.JWT.ValidateAccessToken(u.Data.AccessToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			expiry := time.Unix(isValid.Expiry, 0)
			gracePeriod := expiry.Add(time.Minute * 5)
			currentTime := time.Now()

			// We allow a window period from the moment the access token has
			// expired
			if currentTime.After(gracePeriod) {
				return nil, &ServiceError{ErrMsg: err.Error()}
			}
		} else {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	verified, err := u.JWT.ValidateRefreshToken(u.Data.RefreshToken)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	user, err := u.UserRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		log.FromContext(ctx).WithError(err).Error("failed to find user by id")
		return nil, &ServiceError{ErrMsg: "failed to find user by id", Err: err}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate token")
		return nil, &ServiceError{ErrMsg: "failed to generate token", Err: err}
	}

	err = u.JWT.BlacklistToken(verified, u.Data.RefreshToken)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to blacklist token")
		return nil, &ServiceError{ErrMsg: "failed to blacklist token", Err: err}
	}

	return &token, nil
}
