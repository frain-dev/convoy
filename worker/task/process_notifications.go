package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/util"
	"strconv"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/email"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/hibiken/asynq"

	"github.com/slack-go/slack"
)

var ErrInvalidSlackPayload = errors.New("invalid slack payload")
var ErrInvalidNotificationPayload = errors.New("invalid notification payload")
var ErrInvalidNotificationType = errors.New("invalid notification type")

func ProcessNotifications(sc smtp.SmtpClient) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		n := &notification.Notification{}
		err := util.DecodeMsgPack(t.Payload(), &n)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &n)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		bufP, err := json.Marshal(n.Payload)
		if err != nil {
			return err
		}

		switch n.NotificationType {
		case notification.EmailNotificationType:
			np := &email.Message{}
			err := json.Unmarshal(bufP, np)
			if err != nil {
				return ErrInvalidEmailPayload
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
				return ErrInvalidSlackPayload
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
			return ErrInvalidNotificationType
		}
	}
}
