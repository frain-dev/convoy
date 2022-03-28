package notification

import "context"

type Notification struct {
	Text           string
	Email          string
	LogoURL        string
	TargetURL      string
	EndpointStatus string
}

type Sender interface {
	SendNotification(context.Context, *Notification) error
}
