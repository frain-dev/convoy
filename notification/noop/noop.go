package noop

import (
	"context"

	"github.com/frain-dev/convoy/notification"
)

type Noop struct {
	webhookURL  string
	convoyAgent string
}

func NewNoopNotificationSender() notification.Sender {
	return Noop{}
}

func (s Noop) SendNotification(ctx context.Context, nc *notification.Notification) error {
	return nil
}
