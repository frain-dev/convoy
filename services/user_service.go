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
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (u *UserService) CheckUserExists(ctx context.Context, data *models.UserExists) (bool, error) {
	exists := false
	if err := util.Validate(data); err != nil {
		return exists, NewServiceError(http.StatusBadRequest, err)
	}

	_, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return exists, nil
		}

		return exists, NewServiceError(http.StatusInternalServerError, err)
	}

	exists = true
	return exists, nil
}

func (u *UserService) GeneratePasswordResetToken(ctx context.Context, data *models.GeneratePasswordResetToken) error {
	var resetToken string
	if err := util.Validate(data); err != nil {
		return NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return NewServiceError(http.StatusUnauthorized, errors.New("invalid username"))
		}

		return NewServiceError(http.StatusInternalServerError, err)
	}
	resetToken = uuid.NewString()
	user.ResetPasswordToken = resetToken
	user.ResetPasswordExpiresAt = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour * 1))
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}
	//Todo(Ogban):Send email with token

	return nil
}

func (u *UserService) VerifyPasswordResetToken(ctx context.Context, data *models.VerifyPasswordResetToken) error {
	if err := util.Validate(data); err != nil {
		return NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return NewServiceError(http.StatusUnauthorized, errors.New("invalid username"))
		}
		return NewServiceError(http.StatusInternalServerError, err)
	}
	now := primitive.NewDateTimeFromTime(time.Now())
	if now > user.ResetPasswordExpiresAt {
		return NewServiceError(http.StatusBadRequest, errors.New("password reset token has expired"))
	}
	if data.Token != user.ResetPasswordToken {
		return NewServiceError(http.StatusBadRequest, errors.New("invalid password reset token"))
	}
	return nil
}

func (u *UserService) ResetPassword(ctx context.Context, data *models.ResetPassword) (*datastore.User, error) {

	//Todo: verify token

	if data.Password != data.PasswordConfirmation {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("password confirmation doesn't match password"))
	}

	p := datastore.Password{Plaintext: data.Password}
	err := p.GenerateHash()
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}
	user, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, NewServiceError(http.StatusUnauthorized, errors.New("invalid username"))
		}
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	user.Password = string(p.Hash)
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}
	return user, nil
}
