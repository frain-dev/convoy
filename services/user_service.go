package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
)

type UserService struct {
	userRepo datastore.UserRepository
	cache    cache.Cache
	jwt      *jwt.Jwt
}

func NewUserService(userRepo datastore.UserRepository, cache cache.Cache) (*UserService, error) {
	config, err := config.Get()

	if err != nil {
		return &UserService{}, err
	}

	jwt := jwt.NewJwt(&config.Auth.Native.Jwt, cache)
	return &UserService{userRepo: userRepo, cache: cache, jwt: jwt}, nil
}

func (u *UserService) LoginUser(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
	if err := util.Validate(data); err != nil {
		return nil, nil, NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Username)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, nil, NewServiceError(http.StatusUnauthorized, errors.New("invalid username or password"))
		}

		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}

	p := datastore.Password{Plaintext: data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()

	if err != nil {
		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}
	if !match {
		return nil, nil, NewServiceError(http.StatusUnauthorized, errors.New("invalid username or password"))
	}

	token, err := u.jwt.GenerateToken(user)
	if err != nil {
		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}

	return user, &token, nil

}

func (u *UserService) RefreshToken(ctx context.Context, data *models.Token) (*jwt.Token, error) {
	if err := util.Validate(data); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	isValid, err := u.jwt.ValidateAccessToken(data.AccessToken)
	if err != nil {

		if errors.Is(err, jwt.ErrTokenExpired) {
			expiry := time.Unix(isValid.Expiry, 0)
			gracePeriod := expiry.Add(time.Minute * 5)
			currentTime := time.Now()

			// We allow a window period from the moment the access token has
			// expired
			if currentTime.After(gracePeriod) {
				return nil, NewServiceError(http.StatusUnauthorized, err)
			}
		} else {
			return nil, NewServiceError(http.StatusUnauthorized, err)
		}
	}

	verified, err := u.jwt.ValidateRefreshToken(data.RefreshToken)
	if err != nil {
		return nil, NewServiceError(http.StatusUnauthorized, err)
	}

	user, err := u.userRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, NewServiceError(http.StatusUnauthorized, err)
		}
	}

	token, err := u.jwt.GenerateToken(user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	err = u.jwt.BlacklistToken(verified, data.RefreshToken)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
	}

	return &token, nil

}

func (u *UserService) LogoutUser(token string) error {
	verified, err := u.jwt.ValidateAccessToken(token)
	if err != nil {
		return NewServiceError(http.StatusUnauthorized, err)
	}

	err = u.jwt.BlacklistToken(verified, token)
	if err != nil {
		return NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
	}

	return nil
}
