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

type TemplateName string

const (
	TemplateEndpointUpdate     TemplateName = "endpoint.update"
	TemplateOrganisationInvite TemplateName = "organisation.invite"
	TemplateResetPassword      TemplateName = "reset.password"
)

func (t TemplateName) String() string {
	return string(t)
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
	err := newEmail.Build(n.EmailTemplateName, payload)
	if err != nil {
		return err
	}

	return newEmail.Send(n.Email, "Endpoint Status Update")
}
