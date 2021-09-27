package main

import (
	"bytes"
	"html/template"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg, _ := config.Get()
	smtpClient := New(&cfg.SMTP)

	email := "subomi@frain.dev"
	err := smtpClient.SendEmailNotification(email)
	if err != nil {
		log.WithError(err).Error("Failed to send email")
	}
}

const (
	NotificationTemplate = "endpoint.update.html"
)

type SmtpClient struct {
	url, username, password, from string
}

func New(cfg *config.SMTPConfiguration) *SmtpClient {
	return &SmtpClient{
		url:      cfg.URL,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

func (s *SmtpClient) SendEmailNotification(email string, application *convoy.Application, endpoint *convoy.Endpoint) error {
	// Set up authentication information.
	auth := sasl.NewPlainClient("", s.username, s.password)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	to := []string{application.SupportEmail}

	templ, err := template.ParseFiles(NotificationTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse notification template")
	}

	var body bytes.Buffer
	buildHeaders(s, &body, email)

	templ.Execute(&body, struct {
		Url    string
		Status bool
	}{
		Url:    endpoint.TargetURL,
		Status: true, // endpoint.DocumentStatus,
	})

	data := bytes.NewReader(body.Bytes())
	err = smtp.SendMail(s.url, auth, s.from, to, data)
	if err != nil {
		return err
	}

	return nil
}

func buildHeaders(s *SmtpClient, body *bytes.Buffer, email string) {
	body.Write([]byte(
		"MIME-version: 1.0;" +
			"Content-Type: text/html;" +
			"From: \"Convoy Status\" <" + s.from + ">\r\n" +
			"To: " + email + "\r\n" +
			"Subject: Convoy Endpoint Status Update \r\n" +
			"\r\n",
	))
}
