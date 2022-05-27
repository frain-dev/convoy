package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
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

func NewUserService(userRepo datastore.UserRepository, cache cache.Cache) *UserService {
	return &UserService{userRepo: userRepo, cache: cache}
}

func (u *UserService) LoginUser(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
	if err := util.Validate(data); err != nil {
		return nil, nil, NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Username)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, nil, NewServiceError(http.StatusUnauthorized, err)
		}
	}

	p := datastore.Password{Plaintext: data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()

	if err != nil {
		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}
	if !match {
		return nil, nil, NewServiceError(http.StatusUnauthorized, errors.New("invalid username or password"))
	}

	jwt, err := u.token()
	if err != nil {
		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}

	token, err := jwt.GenerateToken(user)
	if err != nil {
		return nil, nil, NewServiceError(http.StatusInternalServerError, err)
	}

	return user, &token, nil

}

func (u *UserService) RefreshToken(ctx context.Context, data *models.Token) (*jwt.Token, error) {
	var exists *string

	if err := util.Validate(data); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	jw, err := u.token()
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	isValid, err := jw.ValidateAccessToken(data.AccessToken)
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

	// The key stored in the cache is the base64 encoding of the token
	key := convoy.TokenCacheKey.Get(jw.EncodeToken(data.RefreshToken)).String()

	err = u.cache.Get(ctx, key, &exists)
	if err != nil {
		return nil, err
	}

	// If the encoded token exists in the cache, we can safely conclude the
	// refresh token has already been used and has been blacklisted.
	if exists != nil {
		return nil, NewServiceError(http.StatusUnauthorized, errors.New("invalid refresh token"))
	}

	verified, err := jw.ValidateRefreshToken(data.RefreshToken)
	if err != nil {
		return nil, NewServiceError(http.StatusUnauthorized, err)
	}

	user, err := u.userRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, NewServiceError(http.StatusUnauthorized, err)
		}
	}

	token, err := jw.GenerateToken(user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	// Calculate the remaining valid time for the refresh token
	ttl := time.Until(time.Unix(verified.Expiry, 0))
	err = u.cache.Set(ctx, key, &user.UID, ttl)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create token cache"))
	}

	return &token, nil

}

func (u *UserService) token() (*jwt.Jwt, error) {
	if u.jwt != nil {
		return u.jwt, nil
	}

	config, err := config.Get()
	if err != nil {
		return &jwt.Jwt{}, err
	}

	u.jwt = jwt.NewJwt(&config.Auth.Native.Jwt)

	return u.jwt, nil
}
