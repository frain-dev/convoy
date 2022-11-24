package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService struct {
	userRepo      datastore.UserRepository
	cache         cache.Cache
	queue         queue.Queuer
	jwt           *jwt.Jwt
	configService *ConfigService
	orgService    *OrganisationService
}

func NewUserService(userRepo datastore.UserRepository, cache cache.Cache, queue queue.Queuer, configService *ConfigService, orgService *OrganisationService) *UserService {
	return &UserService{userRepo: userRepo, cache: cache, queue: queue, configService: configService, orgService: orgService}
}

func (u *UserService) LoginUser(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
	if err := util.Validate(data); err != nil {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Username)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, nil, util.NewServiceError(http.StatusUnauthorized, errors.New("invalid username or password"))
		}

		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	p := datastore.Password{Plaintext: data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}
	if !match {
		return nil, nil, util.NewServiceError(http.StatusUnauthorized, errors.New("invalid username or password"))
	}

	jwt, err := u.token()
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	token, err := jwt.GenerateToken(user)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return user, &token, nil
}

func (u *UserService) RegisterUser(ctx context.Context, data *models.RegisterUser) (*datastore.User, *jwt.Token, error) {
	var canRegister bool

	if err := util.Validate(data); err != nil {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	config, err := u.configService.LoadConfiguration(ctx)
	if err != nil {
		return nil, nil, err
	}

	if config != nil {
		canRegister = config.IsSignupEnabled
	}

	// registration is not allowed
	if !canRegister {
		return nil, nil, util.NewServiceError(http.StatusForbidden, errors.New("user registration is disabled"))
	}

	p := datastore.Password{Plaintext: data.Password}
	err = p.GenerateHash()

	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	user := &datastore.User{
		UID:       uuid.NewString(),
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		Password:  string(p.Hash),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err = u.userRepo.CreateUser(ctx, user)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			statusCode = http.StatusBadRequest
		}

		return nil, nil, util.NewServiceError(statusCode, err)
	}

	_, err = u.orgService.CreateOrganisation(ctx, &models.Organisation{Name: data.OrganisationName}, user)
	if err != nil {
		return nil, nil, err
	}

	jwt, err := u.token()
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	token, err := jwt.GenerateToken(user)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return user, &token, nil
}

func (u *UserService) RefreshToken(ctx context.Context, data *models.Token) (*jwt.Token, error) {
	if err := util.Validate(data); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	jw, err := u.token()
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
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
				return nil, util.NewServiceError(http.StatusUnauthorized, err)
			}
		} else {
			return nil, util.NewServiceError(http.StatusUnauthorized, err)
		}
	}

	verified, err := jw.ValidateRefreshToken(data.RefreshToken)
	if err != nil {
		return nil, util.NewServiceError(http.StatusUnauthorized, err)
	}

	user, err := u.userRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, util.NewServiceError(http.StatusUnauthorized, err)
		}

		return nil, util.NewServiceError(http.StatusUnauthorized, err)
	}

	token, err := jw.GenerateToken(user)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	err = jw.BlacklistToken(verified, data.RefreshToken)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
	}

	return &token, nil
}

func (u *UserService) LogoutUser(token string) error {
	jw, err := u.token()
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	verified, err := jw.ValidateAccessToken(token)
	if err != nil {
		return util.NewServiceError(http.StatusUnauthorized, err)
	}

	err = jw.BlacklistToken(verified, token)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to blacklist token"))
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
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	user.FirstName = data.FirstName
	user.LastName = data.LastName
	user.Email = data.Email

	err := u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			statusCode = http.StatusBadRequest
		}
		return nil, util.NewServiceError(statusCode, err)
	}

	return user, nil
}

func (u *UserService) UpdatePassword(ctx context.Context, data *models.UpdatePassword, user *datastore.User) (*datastore.User, error) {
	if err := util.Validate(data); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	p := datastore.Password{Plaintext: data.CurrentPassword, Hash: []byte(user.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	if !match {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("current password is invalid"))
	}

	if data.Password != data.PasswordConfirmation {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("password confirmation doesn't match password"))
	}

	p.Plaintext = data.Password
	err = p.GenerateHash()

	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	user.Password = string(p.Hash)
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}

	return user, nil
}

func (u *UserService) GeneratePasswordResetToken(ctx context.Context, baseURL string, data *models.ForgotPassword) error {
	if err := util.Validate(data); err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	user, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return util.NewServiceError(http.StatusBadRequest, errors.New("an account with this email does not exist"))
		}

		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	resetToken := uuid.NewString()
	user.ResetPasswordToken = resetToken
	user.ResetPasswordExpiresAt = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour * 2))
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}
	err = u.sendPasswordResetEmail(ctx, baseURL, resetToken, user)
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	return nil
}

func (u *UserService) sendPasswordResetEmail(ctx context.Context, baseURL string, token string, user *datastore.User) error {
	em := email.Message{
		Email:        user.Email,
		Subject:      "Convoy Password Reset",
		TemplateName: email.TemplateResetPassword,
		Params: map[string]string{
			"password_reset_url": fmt.Sprintf("%s/reset-password?token=%s", baseURL, token),
			"recipient_name":     user.FirstName,
			"expires_at":         user.ResetPasswordExpiresAt.Time().String(),
		},
	}

	buf, err := json.Marshal(em)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to marshal notification payload")
		return err
	}

	job := &queue.Job{
		Payload: json.RawMessage(buf),
		Delay:   0,
	}

	err = u.queue.Write(convoy.EmailProcessor, convoy.DefaultQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to write new notification to the queue")
		return err
	}
	return nil
}

func (u *UserService) ResetPassword(ctx context.Context, token string, data *models.ResetPassword) (*datastore.User, error) {
	user, err := u.userRepo.FindUserByToken(ctx, token)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid password reset token"))
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}
	now := primitive.NewDateTimeFromTime(time.Now())
	if now > user.ResetPasswordExpiresAt {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("password reset token has expired"))
	}
	if data.Password != data.PasswordConfirmation {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("password confirmation doesn't match password"))
	}

	p := datastore.Password{Plaintext: data.Password}
	err = p.GenerateHash()
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	user.Password = string(p.Hash)
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while updating user"))
	}
	return user, nil
}
