package slack

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/notification"
	"github.com/slack-go/slack"
)

type Slack struct {
	webhookURL  string
	convoyAgent string
}

func NewSlackNotificationSender(webhookURL string) notification.Sender {
	s := &Slack{webhookURL: webhookURL}
	s.convoyAgent = "Convoy/" + convoy.GetVersion()
	return s
}

func (s *Slack) SendNotification(ctx context.Context, nc *notification.Notification) error {
	attachment := slack.Attachment{
		AuthorName: s.convoyAgent,
		Text:       nc.Text,
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}

	msg := &slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}

	err := slack.PostWebhookContext(ctx, s.webhookURL, msg)
	if err != nil {
		return err
	}
	return nil
}
