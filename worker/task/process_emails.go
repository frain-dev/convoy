package task

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
)

var ErrInvalidEmailPayload = errors.New("invalid email payload")

func ProcessEmails(sc smtp.SmtpClient) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var message email.Message
		if err := json.Unmarshal(t.Payload(), &message); err != nil {
			log.WithError(err).Error("Failed to unmarshal email message payload")
			return ErrInvalidEmailPayload
		}

		newEmail := email.NewEmail(sc)

		if err := newEmail.Build(string(message.TemplateName), message.Params); err != nil {
			log.WithError(err).Error("Failed to build email")
			return err
		}

		if err := newEmail.Send(message.Email, message.Subject); err != nil {
			log.WithError(err).Error("Failed to send email")
			return err
		}

		return nil
	}
}
