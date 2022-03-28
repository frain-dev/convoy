package noop

import (
	"context"

	"github.com/frain-dev/convoy/notification"
)

type Noop struct{}

func NewNoopNotificationSender() notification.Sender {
	return Noop{}
}

func (s Noop) SendNotification(ctx context.Context, nc *notification.Notification) error {
	return nil
}
