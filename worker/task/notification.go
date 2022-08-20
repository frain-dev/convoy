package task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/notifications"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
)

func sendNotification(
	ctx context.Context,
	appRepo datastore.ApplicationRepository,
	eventDelivery *datastore.EventDelivery,
	g *datastore.Group,
	smtpCfg *config.SMTPConfiguration,
	status datastore.SubscriptionStatus,
	q queue.Queuer,
	failure bool,
) error {
	app, err := appRepo.FindApplicationByID(ctx, eventDelivery.AppID)
	if err != nil {
		return fmt.Errorf("failed to fetch application: %v", err)
	}

	endpoint, err := appRepo.FindApplicationEndpointByID(ctx, eventDelivery.AppID, eventDelivery.EndpointID)
	if err != nil {
		return fmt.Errorf("failed to fetch application endpoint: %v", err)
	}

	if !util.IsStringEmpty(app.SupportEmail) {
		n := &notification.Notification{
			NotificationType: notifications.EmailNotificationType,
			Payload: email.Message{
				Email:        app.SupportEmail,
				Subject:      "Endpoint Status Update",
				TemplateName: email.TemplateEndpointUpdate,
				Params: map[string]string{
					"logo_url":        g.LogoURL,
					"target_url":      endpoint.TargetURL,
					"endpoint_status": string(status),
				},
			},
		}

		buf, err := json.Marshal(n)
		if err != nil {
			log.WithError(err).Error("Failed to marshal email notification payload")
			return nil
		}

		job := &queue.Job{
			Payload: json.RawMessage(buf),
			Delay:   0,
		}
		err = q.Write(convoy.NotificationProcessor, convoy.DefaultQueue, job)
		if err != nil {
			log.WithError(err).Error("Failed to write new notification to the queue")
		}

		return nil
	}

	if !util.IsStringEmpty(app.SlackWebhookURL) {
		n := &notification.Notification{
			NotificationType: notifications.SlackNotificationType,
		}

		payload := notification.SlackNotification{
			WebhookURL: app.SlackWebhookURL,
		}

		var text string
		if failure {
			text = fmt.Sprintf("failed to send event delivery (%s) to endpoint url (%s) after retry limit was hit, endpoint status is now %s", eventDelivery.UID, endpoint.TargetURL, status)
		} else {
			text = fmt.Sprintf("endpoint url (%s) which was formerly dectivated has now been reactivated, endpoint status is now %s", endpoint.TargetURL, status)
		}

		payload.Text = text
		n.Payload = payload

		buf, err := json.Marshal(n)
		if err != nil {
			log.WithError(err).Error("Failed to marshal slack notification payload")
			return nil
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
