package task

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/olamilekan000/surge/surge/job"

	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

var ErrInvalidEmailPayload = errors.New("invalid email payload")

func ProcessEmails(sc smtp.SmtpClient) func(context.Context, *job.JobEnvelope) error {
	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
		var message email.Message

		err := msgpack.DecodeMsgPack(jobEnvelope.Args, &message)
		if err != nil {
			err := json.Unmarshal(jobEnvelope.Args, &message)
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
