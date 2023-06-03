package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type UserService struct {
	userRepo      datastore.UserRepository
	orgRepo       datastore.OrganisationRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	cache         cache.Cache
	queue         queue.Queuer
	jwt           *jwt.Jwt
	configService *ConfigService
}

func NewUserService(userRepo datastore.UserRepository, cache cache.Cache, queue queue.Queuer, configService *ConfigService, orgRepo datastore.OrganisationRepository, orgMemberRepo datastore.OrganisationMemberRepository) *UserService {
	return &UserService{userRepo: userRepo, cache: cache, queue: queue, configService: configService, orgMemberRepo: orgMemberRepo, orgRepo: orgRepo}
}

func (u *UserService) LoginUser(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
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

func (u *UserService) RegisterUser(ctx context.Context, baseURL string, data *models.RegisterUser) (*datastore.User, *jwt.Token, error) {
	config, err := u.configService.LoadConfiguration(ctx)
	if err != nil {
		return nil, nil, err
	}

	if config != nil {
		if !config.IsSignupEnabled {
			// registration is not allowed
			return nil, nil, util.NewServiceError(http.StatusForbidden, errors.New("user registration is disabled"))
		}
	}

	p := datastore.Password{Plaintext: data.Password}
	err = p.GenerateHash()

	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	user := &datastore.User{
		UID:                        ulid.Make().String(),
		FirstName:                  data.FirstName,
		LastName:                   data.LastName,
		Email:                      data.Email,
		Password:                   string(p.Hash),
		EmailVerificationToken:     ulid.Make().String(),
		CreatedAt:                  time.Now(),
		UpdatedAt:                  time.Now(),
		EmailVerificationExpiresAt: time.Now().Add(time.Hour * 2),
	}

	err = u.userRepo.CreateUser(ctx, user)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			statusCode = http.StatusBadRequest
		}

		return nil, nil, util.NewServiceError(statusCode, err)
	}

	co := CreateOrganisationService{
		OrgRepo:       u.orgRepo,
		OrgMemberRepo: u.orgMemberRepo,
		NewOrg:        &models.Organisation{Name: data.OrganisationName},
		User:          user,
	}

	_, err = co.Run(ctx)
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

	err = u.sendUserVerificationEmail(ctx, baseURL, user)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return user, &token, nil
}

func (u *UserService) ResendEmailVerificationToken(ctx context.Context, baseURL string, user *datastore.User) error {
	if user.EmailVerified {
		return util.NewServiceError(http.StatusBadRequest, errors.New("user email already verified"))
	}

	if user.EmailVerificationExpiresAt.After(time.Now()) {
		return util.NewServiceError(http.StatusBadRequest, errors.New("old verification token is still valid"))
	}

	user.EmailVerificationExpiresAt = time.Now().Add(time.Hour * 2)
	user.EmailVerificationToken = ulid.Make().String()

	err := u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update user")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update user"))
	}

	err = u.sendUserVerificationEmail(ctx, baseURL, user)
	if err != nil {
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

func (u *UserService) RefreshToken(ctx context.Context, data *models.Token) (*jwt.Token, error) {
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

func (u *UserService) sendUserVerificationEmail(ctx context.Context, baseURL string, user *datastore.User) error {
	em := email.Message{
		Email:        user.Email,
		Subject:      "Convoy Email Verification",
		TemplateName: email.TemplateEmailVerification,
		Params: map[string]string{
			"email_verification_url": fmt.Sprintf("%s/verify-email?verification-token=%s", baseURL, user.EmailVerificationToken),
			"recipient_name":         user.FirstName,
			"email":                  user.Email,
			"expires_at":             user.EmailVerificationExpiresAt.String(),
		},
	}

	return u.queueEmail(ctx, &em)
}

func (u *UserService) queueEmail(ctx context.Context, em *email.Message) error {
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

func (u *UserService) VerifyEmail(ctx context.Context, token string) error {
	user, err := u.userRepo.FindUserByEmailVerificationToken(ctx, token)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return util.NewServiceError(http.StatusBadRequest, errors.New("invalid password reset token"))
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if time.Now().After(user.EmailVerificationExpiresAt) {
		return util.NewServiceError(http.StatusBadRequest, errors.New("email verification token has expired"))
	}

	user.EmailVerified = true
	err = u.userRepo.UpdateUser(ctx, user)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			statusCode = http.StatusBadRequest
		}
		return util.NewServiceError(statusCode, err)
	}

	return nil
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
	if !user.EmailVerified {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("email has not been verified"))
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
	user, err := u.userRepo.FindUserByEmail(ctx, data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return util.NewServiceError(http.StatusBadRequest, errors.New("an account with this email does not exist"))
		}

		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	resetToken := ulid.Make().String()
	user.ResetPasswordToken = resetToken
	user.ResetPasswordExpiresAt = time.Now().Add(time.Hour * 2)
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
			"password_reset_url": fmt.Sprintf("%s/reset-password?auth-token=%s", baseURL, token),
			"recipient_name":     user.FirstName,
			"expires_at":         user.ResetPasswordExpiresAt.String(),
		},
	}

	return u.queueEmail(ctx, &em)
}

func (u *UserService) ResetPassword(ctx context.Context, token string, data *models.ResetPassword) (*datastore.User, error) {
	user, err := u.userRepo.FindUserByToken(ctx, token)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("invalid password reset token"))
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	if time.Now().After(user.ResetPasswordExpiresAt) {
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
