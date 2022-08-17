package notifications

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
)

// Sender defines the method set any notifier should contain.
type Sender interface {
	SendNotification(context.Context, interface{}) error
}

type NotificationType string

const (
	SlackNotificationType NotificationType = "slack"
	EmailNotificationType NotificationType = "email"
)

type Notification struct {
	// Defines the type of notification either slack or email.
	NotificationType NotificationType

	// Email or Slack notification
	Payload interface{}
}

type SlackNotification struct {
	WebhookURL string `json:"webhook_url,omitempty"`

	Text string `json:"text,omitempty"`
}

// ProcessNotification is the entrypoint to this package. It processes
// each notifications with the correct handler.
func ProcessNotification(ctx context.Context, sC smtp.Client, payload interface{}) error {
	return errors.New("Function not implemented")
}

// EMAIL NOTIFICATION
func SendEmailNotification(ctx context.Context, n Notification) error {
	newEmail := email.NewEmail(s)
	err := newEmail.Build(n.EmailTemplateName, n)
	if err != nil {
		return err
	}

	return newEmail.Send(n.Email, n.Subject)
}

// SLACK NOTIFICATION
func SendSlackNotification(ctx context.Context, notification SlackNotification) error {
	return errors.New("Function not implemented")
}
