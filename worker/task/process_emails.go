package task

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/hibiken/asynq"
)

var ErrInvalidEmailPayload = errors.New("invalid email payload")

func ProcessEmails(sc smtp.SmtpClient) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var message email.Message

		err := util.DecodeMsgPack(t.Payload(), &message)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &message)
			if err != nil {
				return ErrInvalidEmailPayload
			}
		}

		newEmail := email.NewEmail(sc)

		if err := newEmail.Build(string(message.TemplateName), message.Params); err != nil {
			return err
		}

		if err := newEmail.Send(message.Email, message.Subject); err != nil {
			return err
		}

		return nil
	}
}
