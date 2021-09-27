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
	smtpClient := New()

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

func New(cfg *config.SmtpConfiguration) *SmtpClient {
	return &SmtpClient{
		url:      cfg.URL,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

func (s *SmtpClient) SendEmailNotification(email string, endpoint *convoy.Endpoint) error {
	// Set up authentication information.
	auth := sasl.NewPlainClient("", s.username, s.password)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	to := []string{email}

	templ, err := template.ParseFiles(NotificationTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse notification template")
	}

	var body bytes.Buffer
	s.buildHeaders(&body, email)

	templ.Execute(&body, struct {
		Url    string
		Status bool
	}{
		Url:    endpoint.TargetURL,
		Status: endpoint.DocumentStatus,
	})

	err = smtp.SendMail(s.url, auth, s.from, to, body)
	if err != nil {
		return err
	}

	return nil
}

func (s *SmtpClient) buildHeaders(body *bytes.Buffer, email string) {
	body.Write([]byte(
		"MIME-version: 1.0;" +
			"Content-Type: text/html;" +
			"From: \"Convoy Status\" <" + s.from + ">\r\n" +
			"To: " + email + "\r\n" +
			"Subject: Convoy Endpoint Status Update \r\n" +
			"\r\n",
	))
}
