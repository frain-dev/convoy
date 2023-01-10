package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/frain-dev/convoy/internal/pkg/smtp"
)

//go:embed templates/*
var templateDir embed.FS

const (
	templatePath = "templates/"
	fileSuffix   = ".html"
)

type TemplateName string

const (
	TemplateEndpointUpdate     TemplateName = "endpoint.update"
	TemplateEmailVerification  TemplateName = "user.verify.email"
	TemplateOrganisationInvite TemplateName = "organisation.invite"
	TemplateResetPassword      TemplateName = "reset.password"
	TemplateTwitterSource      TemplateName = "twitter.source"
)

func (t TemplateName) String() string {
	return string(t)
}

// Message represents a generic email message. It can be anything from
// an organisation invite to a disabled endpoint email
type Message struct {
	// Email represents email recipient.
	Email string `json:"email,omitempty"`

	// Subject of the email
	Subject string `json:"subject,omitempty"`

	// Params to replace the template variables.
	Params interface{} `json:"params,omitempty"`

	// Glob represents which template to use in building the email
	TemplateName TemplateName `json:"template_name,omitempty"`
}

type Email struct {
	client smtp.SmtpClient
	templ  *template.Template
	body   bytes.Buffer
}

func NewEmail(c smtp.SmtpClient) *Email {
	return &Email{client: c}
}

// TODO(subomi): glob pattern must not match more than one template
func (e *Email) Build(glob string, params interface{}) error {
	templ, err := e.templ.ParseFS(templateDir, e.buildGlob(glob))
	if err != nil {
		return fmt.Errorf("failed to parse template fs: %v", err)
	}
	e.templ = templ

	err = e.templ.Execute(&e.body, params)
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return nil
}

func (e *Email) Send(emailAddr, subject string) error {
	err := e.client.SendEmail(emailAddr, subject, e.body)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

func (e *Email) buildGlob(glob string) string {
	var s strings.Builder

	s.WriteString(templatePath)
	s.WriteString(glob)

	if !strings.HasSuffix(glob, fileSuffix) {
		s.WriteString(fileSuffix)
	}

	return s.String()
}
