package notifications

import (
	"context"
	"errors"

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
	NotificationType NotificationType `json:"notification_type,omitempty"`

	// Email or Slack notification
	Payload interface{} `json:"payload,omitempty"`
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
	return errors.New("Function not implemented")
}

// SLACK NOTIFICATION
func SendSlackNotification(ctx context.Context, notification SlackNotification) error {
	return errors.New("Function not implemented")
}
