package notification

import "context"

type Notification struct {
	Text string
}

type Sender interface {
	SendNotification(context.Context, *Notification) error
}
