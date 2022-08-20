package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/email"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

func ProcessNotifications(ctx context.Context, t *asynq.Task) error {
	buf := t.Payload()

	n := &notification.Notification{}
	err := json.Unmarshal(buf, n)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal notification payload")
		return &EndpointError{Err: err, delay: defaultDelay}
	}

	bufP, err := json.Marshal(n.Payload)
	if err != nil {
		log.WithError(err).Error("Failed to marshal payload")
		return err
	}

	switch n.NotificationType {
	case notification.EmailNotificationType:
		np := &email.Message{}
		err := json.Unmarshal(bufP, np)
		if err != nil {
			log.WithError(err).Error("Failed to unmarshal email notification payload")
			return err
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
		err = newEmail.Build(string(np.TemplateName), np.Params)
		if err != nil {
			return err
		}

		return newEmail.Send(np.Email, np.Subject)
	case notification.SlackNotificationType:
		np := &notification.SlackNotification{}
		err := json.Unmarshal(bufP, np)
		if err != nil {
			log.WithError(err).Error("Failed to unmarshal email notification payload")
			return err
		}

		convoyAgent := fmt.Sprintf("Convoy/%s", convoy.GetVersion())
		attachment := slack.Attachment{
			AuthorName: convoyAgent,
			Text:       np.Text,
			Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		}

		msg := &slack.WebhookMessage{
			Attachments: []slack.Attachment{attachment},
		}

		err = slack.PostWebhookContext(ctx, np.WebhookURL, msg)
		if err != nil {
			return err
		}
		return nil

	default:
		return errors.New("Invalid notification type")
	}
}
