package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/olamilekan000/surge/surge/job"
	"github.com/slack-go/slack"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/email"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

var ErrInvalidSlackPayload = errors.New("invalid slack payload")
var ErrInvalidNotificationPayload = errors.New("invalid notification payload")
var ErrInvalidNotificationType = errors.New("invalid notification type")

func ProcessNotifications(sc smtp.SmtpClient) func(context.Context, *job.JobEnvelope) error {
	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
		n := &notification.Notification{}
		err := msgpack.DecodeMsgPack(jobEnvelope.Args, &n)
		if err != nil {
			err := json.Unmarshal(jobEnvelope.Args, &n)
			if err != nil {
				// If unmarshal fails, try parsing as raw email.Message (backward compatibility)
				np := &email.Message{}
				err := msgpack.DecodeMsgPack(jobEnvelope.Args, np)
				if err != nil {
					err := json.Unmarshal(jobEnvelope.Args, np)
					if err != nil {
						return ErrInvalidNotificationPayload
					}
				}
				// Successfully parsed as email, process it
				if np.Email != "" {
					newEmail := email.NewEmail(sc)
					err = newEmail.Build(string(np.TemplateName), np.Params)
					if err != nil {
						return err
					}
					return newEmail.Send(np.Email, np.Subject)
				}
				return ErrInvalidNotificationPayload
			}
		}

		// If NotificationType is empty and Payload is nil/empty, try parsing original payload as raw email.Message
		payloadEmpty := n.Payload == nil
		if !payloadEmpty {
			// Check if payload is an empty map/interface
			if payloadMap, ok := n.Payload.(map[string]interface{}); ok && len(payloadMap) == 0 {
				payloadEmpty = true
			}
		}
		if n.NotificationType == "" && payloadEmpty {
			np := &email.Message{}
			err := msgpack.DecodeMsgPack(jobEnvelope.Args, np)
			if err != nil {
				err := json.Unmarshal(jobEnvelope.Args, np)
				if err != nil {
					return ErrInvalidNotificationPayload
				}
			}
			// Successfully parsed as email, process it
			if np.Email != "" {
				newEmail := email.NewEmail(sc)
				err = newEmail.Build(string(np.TemplateName), np.Params)
				if err != nil {
					return err
				}
				return newEmail.Send(np.Email, np.Subject)
			}
			return ErrInvalidNotificationPayload
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
			// Default to email if notification type is empty/invalid but payload can be parsed as email
			np := &email.Message{}
			err := json.Unmarshal(bufP, np)
			if err == nil && np.Email != "" {
				// Successfully parsed as email, process it
				newEmail := email.NewEmail(sc)
				err = newEmail.Build(string(np.TemplateName), np.Params)
				if err != nil {
					return err
				}
				return newEmail.Send(np.Email, np.Subject)
			}

			return ErrInvalidNotificationType
		}
	}
}
