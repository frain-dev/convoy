package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/notification"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func ProcessNotification(ctx context.Context, t *asynq.Task) error {
	buf := t.Payload()

	n := &notification.Notification{}
	err := json.Unmarshal(buf, n)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal notification payload")
		return &EndpointError{Err: err, delay: defaultDelay}
	}

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Error("Failed to load config")
		return err
	}

	switch n.NotificationType {
	case notification.EmailNotificationType:
		sc, err := smtp.New(&cfg.SMTP)
		if err != nil {
			log.WithError(err).Error("Failed to create smtp client")
			return err
		}

		newEmail := email.NewEmail(sc)
		msg, ok := n.(email.Message)
		if !ok {
			return errors.New("Invalid email message type")
		}

		err = newEmail.Build(msg.Glob, msg.Params)
		if err != nil {
			return err
		}

		return newEmail.Send(msg.Email, msg.Subject)

	case notification.SlackNotificationType:
		fmt.Println("Processing slack notification")
	default:
		return errors.New("Invalid notification type")
	}

	return nil
}
