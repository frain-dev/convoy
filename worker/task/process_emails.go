package task

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func ProcessEmails(ctx context.Context, t *asynq.Task) error {
	var message email.Message
	if err := json.Unmarshal(t.Payload(), &message); err != nil {
		log.WithError(err).Error("Failed to unmarshal email message payload")
	}

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Error("Failed to load config")
		return err
	}

	sc, err := smtp.New(&cfg.SMTP)
	if err != nil {
		log.WithError(err).Error("Failed to create smtp client")
		return err
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
