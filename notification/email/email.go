package email

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/smtp"
)

type Email struct {
	s *smtp.SmtpClient
}

func NewEmailNotificationSender(smtpCfg *config.SMTPConfiguration) (notification.Sender, error) {
	s, err := smtp.New(smtpCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize smtp client: %v", err)
	}

	return &Email{s: s}, nil
}

func (e *Email) SendNotification(ctx context.Context, n *notification.Notification) error {
	return e.s.SendEmailNotification(n.Email, n.LogoURL, n.TargetURL, n.EndpointStatus)
}
