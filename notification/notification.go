package notification

import "context"

type Notification struct {
	Text              string
	Email             string
	LogoURL           string
	TargetURL         string
	EndpointStatus    string
	EmailTemplateName string
	InviteURL         string
	OrganisationName  string
	InviterName       string
}

type Sender interface {
	SendNotification(context.Context, *Notification) error
}
