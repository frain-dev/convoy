package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
)

type NotificationType string

const (
	SlackNotificationType NotificationType = "slack"
	EmailNotificationType NotificationType = "email"
)

type Notification struct {
	// Defines the type of notification either slack or email.
	NotificationType NotificationType `json:"notification_type,omitempty"`

	// Email or Slack notification
	Payload interface{} `json:"payload,omitempty"`
}

type SlackNotification struct {
	WebhookURL string `json:"webhook_url,omitempty"`

	Text string `json:"text,omitempty"`
}

// NOTIFICATIONS

func SendEndpointNotification(ctx context.Context,
	app *datastore.Application,
	endpoint *datastore.Endpoint,
	group *datastore.Group,
	status datastore.SubscriptionStatus,
	q queue.Queuer,
	failure bool,
	failureMsg string,
) error {
	var ns []*Notification

	if !util.IsStringEmpty(app.SupportEmail) {
		ns = append(ns, &Notification{NotificationType: EmailNotificationType})
	}

	if !util.IsStringEmpty(app.SlackWebhookURL) {
		ns = append(ns, &Notification{NotificationType: SlackNotificationType})
	}

	for _, v := range ns {
		switch v.NotificationType {
		case EmailNotificationType:
			v.Payload = email.Message{
				Email:        app.SupportEmail,
				Subject:      "Endpoint Status Update",
				TemplateName: email.TemplateEndpointUpdate,
				Params: map[string]string{
					"logo_url":        group.LogoURL,
					"target_url":      endpoint.TargetURL,
					"failure_msg":     failureMsg,
					"endpoint_status": string(status),
				},
			}
		case SlackNotificationType:
			payload := SlackNotification{
				WebhookURL: app.SlackWebhookURL,
			}

			var text string
			if failure {
				text = fmt.Sprintf("failed to send event delivery to endpoint url (%s) after retry limit was hit, reason for failure is \"%s\", endpoint status is now %s", endpoint.TargetURL, failureMsg, status)
			} else {
				text = fmt.Sprintf("endpoint url (%s) which was formerly dectivated has now been reactivated, endpoint status is now %s", endpoint.TargetURL, status)
			}

			payload.Text = text
			v.Payload = payload
		default:
			log.Error("Invalid notification type")
			continue
		}

		buf, err := json.Marshal(v)
		if err != nil {
			log.WithError(err).Errorf("Failed to marshal %v notification payload", v.NotificationType)
			continue
		}

		job := &queue.Job{
			Payload: json.RawMessage(buf),
			Delay:   0,
		}

		err = q.Write(convoy.NotificationProcessor, convoy.DefaultQueue, job)
		if err != nil {
			log.WithError(err).Error("Failed to write new notification to the queue")
		}
	}

	return nil
}
