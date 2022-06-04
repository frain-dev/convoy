package email

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/config"
	em "github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/pkg/smtp"
)

type Email struct {
	s smtp.SmtpClient
}

func NewEmailNotificationSender(smtpCfg *config.SMTPConfiguration) (notification.Sender, error) {
	s, err := smtp.New(smtpCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize smtp client: %v", err)
	}

	return &Email{s: s}, nil
}

func (e *Email) SendNotification(ctx context.Context, n *notification.Notification) error {
	payload := struct {
		URL     string
		LogoURL string
		Status  string
	}{
		URL:     n.TargetURL,
		LogoURL: n.LogoURL,
		Status:  n.EndpointStatus,
	}

	newEmail := em.NewEmail(e.s)
	err := newEmail.Build("endpoint.update", payload)
	if err != nil {
		return err
	}

	return newEmail.Send(n.Email, "Endpoint Status Update")
}
