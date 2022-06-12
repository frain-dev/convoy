package email

import (
	"bytes"
	"embed"
	"html/template"
	"strings"

	"github.com/frain-dev/convoy/pkg/smtp"
)

//go:embed templates/*
var templateDir embed.FS

const (
	templatePath = "templates/"
	fileSuffix   = ".html"
)

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
		return err
	}
	e.templ = templ

	err = e.templ.Execute(&e.body, params)
	if err != nil {
		return err
	}

	return nil
}

func (e *Email) Send(emailAddr, subject string) error {
	err := e.client.SendEmail(emailAddr, subject, e.body)
	if err != nil {
		return err
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
