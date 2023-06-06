package services

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/queue"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

type GeneratePasswordResetTokenService struct {
	UserRepo datastore.UserRepository
	Queue    queue.Queuer

	BaseURL string
	Data    *models.ForgotPassword
}

func (u *GeneratePasswordResetTokenService) Run(ctx context.Context) error {
	user, err := u.UserRepo.FindUserByEmail(ctx, u.Data.Email)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return &ServiceError{ErrMsg: "an account with this email does not exist"}
		}

		log.FromContext(ctx).WithError(err).Error("failed to find user by email")
		return &ServiceError{ErrMsg: "failed to find user by email", Err: err}
	}

	resetToken := ulid.Make().String()
	user.ResetPasswordToken = resetToken
	user.ResetPasswordExpiresAt = time.Now().Add(time.Hour * 2)

	err = u.UserRepo.UpdateUser(ctx, user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update user")
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	err = u.sendPasswordResetEmail(ctx, u.BaseURL, resetToken, user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to queue password reset email")
		return &ServiceError{ErrMsg: err.Error()}
	}
	return nil
}

func (u *GeneratePasswordResetTokenService) sendPasswordResetEmail(ctx context.Context, baseURL string, token string, user *datastore.User) error {
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

	return queueEmail(ctx, &em, u.Queue)
}
