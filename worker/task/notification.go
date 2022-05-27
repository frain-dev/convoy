package task

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/notification/email"
	"github.com/frain-dev/convoy/notification/slack"
)

func sendNotification(
	ctx context.Context,
	appRepo datastore.ApplicationRepository,
	eventDelivery *datastore.EventDelivery,
	g *datastore.Group,
	smtpCfg *config.SMTPConfiguration,
	status datastore.EndpointStatus,
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

	n := &notification.Notification{
		LogoURL:        g.LogoURL,
		TargetURL:      endpoint.TargetURL,
		EndpointStatus: string(status),
	}

	if failure {
		n.Text = fmt.Sprintf("failed to send event delivery (%s) to endpoint url (%s) after retry limit was hit, endpoint status is now %s", eventDelivery.UID, endpoint.TargetURL, status)
	} else {
		n.Text = fmt.Sprintf("endpoint url (%s) which was formerly dectivated has now been reactivated, endpoint status is now %s", endpoint.TargetURL, status)
	}

	if !util.IsStringEmpty(app.SupportEmail) {
		err = sendEmailNotification(ctx, n, smtpCfg)
		if err != nil {
			return fmt.Errorf("failed to send slack notification: %v", err)
		}
	}

	if !util.IsStringEmpty(app.SlackWebhookURL) {
		err = sendSlackNotification(ctx, app.SlackWebhookURL, n)
		if err != nil {
			return fmt.Errorf("failed to send email notification: %v", err)
		}
	}

	return nil
}

func sendEmailNotification(ctx context.Context, n *notification.Notification, smtpCfg *config.SMTPConfiguration) error {
	em, err := email.NewEmailNotificationSender(smtpCfg)
	if err != nil {
		return fmt.Errorf("failed to get new email notification sender: %v", err)
	}

	err = em.SendNotification(ctx, n)
	if err != nil {
		return fmt.Errorf("failed to send email notification: %v", err)
	}

	return nil
}

func sendSlackNotification(ctx context.Context, slackWebhookURL string, n *notification.Notification) error {
	err := slack.NewSlackNotificationSender(slackWebhookURL).SendNotification(ctx, n)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %v", err)
	}
	return nil
}
