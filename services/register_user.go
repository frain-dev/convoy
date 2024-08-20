package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
)

type RegisterUserService struct {
	UserRepo      datastore.UserRepository
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Queue         queue.Queuer
	JWT           *jwt.Jwt
	ConfigRepo    datastore.ConfigurationRepository
	Licenser      license.Licenser

	BaseURL string
	Data    *models.RegisterUser
}

func (u *RegisterUserService) Run(ctx context.Context) (*datastore.User, *jwt.Token, error) {
	ok, err := u.Licenser.CreateUser(ctx)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !ok {
		return nil, nil, &ServiceError{ErrMsg: ErrUserLimit.Error()}
	}

	config, err := u.ConfigRepo.LoadConfiguration(ctx)
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		return nil, nil, &ServiceError{ErrMsg: "failed to load configuration", Err: err}
	}

	if config != nil {
		if !config.IsSignupEnabled {
			// registration is not allowed
			return nil, nil, &ServiceError{ErrMsg: datastore.ErrSignupDisabled.Error(), Err: datastore.ErrSignupDisabled}
		}
	}

	p := datastore.Password{Plaintext: u.Data.Password}
	err = p.GenerateHash()

	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate hash")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate hash", Err: err}
	}

	user := &datastore.User{
		UID:                        ulid.Make().String(),
		FirstName:                  u.Data.FirstName,
		LastName:                   u.Data.LastName,
		Email:                      u.Data.Email,
		Password:                   string(p.Hash),
		EmailVerificationToken:     ulid.Make().String(),
		CreatedAt:                  time.Now(),
		UpdatedAt:                  time.Now(),
		EmailVerificationExpiresAt: time.Now().Add(time.Hour * 2),
	}

	err = u.UserRepo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			return nil, nil, &ServiceError{ErrMsg: "this email is taken"}
		}

		log.FromContext(ctx).WithError(err).Error("failed to create user")
		return nil, nil, &ServiceError{ErrMsg: "failed to create user", Err: err}
	}

	co := CreateOrganisationService{
		OrgRepo:       u.OrgRepo,
		OrgMemberRepo: u.OrgMemberRepo,
		Licenser:      u.Licenser,
		NewOrg:        &models.Organisation{Name: u.Data.OrganisationName},
		User:          user,
	}

	_, err = co.Run(ctx)
	if err != nil {
		if !errors.Is(err, ErrOrgLimit) && !errors.Is(err, ErrUserLimit) {
			return nil, nil, err
		}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate token")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate token", Err: err}
	}

	err = sendUserVerificationEmail(ctx, u.BaseURL, user, u.Queue)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
	}

	return user, &token, nil
}

func sendUserVerificationEmail(ctx context.Context, baseURL string, user *datastore.User, q queue.Queuer) error {
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

	return queueEmail(ctx, &em, q)
}

func queueEmail(ctx context.Context, em *email.Message, q queue.Queuer) error {
	bytes, err := msgpack.EncodeMsgPack(em)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to marshal notification payload")
		return err
	}

	job := &queue.Job{
		Payload: bytes,
		Delay:   0,
	}

	err = q.Write(convoy.EmailProcessor, convoy.DefaultQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to write new notification to the queue")
		return err
	}
	return nil
}
