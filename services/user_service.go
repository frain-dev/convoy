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

	verified, err := jw.ValidateRefreshToken(data.RefreshToken)
	if err != nil {
		return nil, NewServiceError(http.StatusUnauthorized, err)
	}

	user, err := u.userRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, NewServiceError(http.StatusUnauthorized, err)
		}

		return nil, NewServiceError(http.StatusUnauthorized, err)
	}

	token, err := jw.GenerateToken(user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	err = jw.BlacklistToken(verified, data.RefreshToken)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
	}

	return &token, nil

}

func (u *UserService) LogoutUser(token string) error {
	jw, err := u.token()
	if err != nil {
		return NewServiceError(http.StatusInternalServerError, err)
	}

	verified, err := jw.ValidateAccessToken(token)
	if err != nil {
		return NewServiceError(http.StatusUnauthorized, err)
	}

	err = jw.BlacklistToken(verified, token)
	if err != nil {
		return NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
	}

	return nil
}

func (u *UserService) token() (*jwt.Jwt, error) {
	if u.jwt != nil {
		return u.jwt, nil
	}

	config, err := config.Get()
	if err != nil {
		return &jwt.Jwt{}, err
	}

	u.jwt = jwt.NewJwt(&config.Auth.Jwt, u.cache)
	return u.jwt, nil
}

func (u *UserService) UpdateUser(ctx context.Context, data *models.UpdateUser, user *datastore.User) (*datastore.User, error) {
	if err := util.Validate(data); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	user.FirstName = data.FirstName
	user.LastName = data.LastName
	user.Email = data.Email

	err := u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}

	return user, nil
}

func (u *UserService) UpdatePassword(ctx context.Context, data *models.UpdatePassword, user *datastore.User) (*datastore.User, error) {
	if err := util.Validate(data); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	p := datastore.Password{Plaintext: data.CurrentPassword, Hash: []byte(user.Password)}
	match, err := p.Matches()

	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	if !match {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("current password is invalid"))
	}

	if data.Password != data.PasswordConfirmation {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("password confirmation doesn't match password"))
	}

	p.Plaintext = data.Password
	err = p.GenerateHash()

	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	user.Password = string(p.Hash)
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}

	return user, nil
}
