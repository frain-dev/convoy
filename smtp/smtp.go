package smtp

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
)

const (
	NotificationTemplate = "endpoint.update.html"
)

type SmtpClient struct {
	url, username, password, from string
}

func New(cfg *config.SMTPConfiguration) (*SmtpClient, error) {
	var err error

	errMsg := "Missing SMTP Config - %s"
	if util.IsStringEmpty(cfg.URL) {
		err = fmt.Errorf(errMsg, "URL")
		log.WithError(err).Error()
	}

	if util.IsStringEmpty(cfg.Username) {
		err = fmt.Errorf(errMsg, "username")
		log.WithError(err).Error()
	}

	if util.IsStringEmpty(cfg.Password) {
		err = fmt.Errorf(errMsg, "password")
		log.WithError(err).Error()
	}

	if util.IsStringEmpty(cfg.From) {
		err = fmt.Errorf(errMsg, "from")
		log.WithError(err).Error()
	}

	return &SmtpClient{
		url:      cfg.URL,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}, err
}

func (s *SmtpClient) SendEmailNotification(email string, endpoint convoy.EndpointMetadata) error {
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
	buildHeaders(s, &body, email)

	err = templ.Execute(&body, struct {
		URL      string
		Disabled bool
	}{
		URL:      endpoint.TargetURL,
		Disabled: endpoint.Disabled,
	})

	if err != nil {
		log.WithError(err).Error("Failed to build template")
		return err
	}

	data := bytes.NewReader(body.Bytes())
	err = smtp.SendMail(s.url, auth, s.from, to, data)
	if err != nil {
		log.WithError(err).Error("Failed to send email notification")
		return err
	}

	return nil
}

func buildHeaders(s *SmtpClient, body *bytes.Buffer, email string) {
	body.Write([]byte(
		"MIME-version: 1.0;\n" +
			"Content-Type: text/html;\r\n" +
			"From: \"Convoy Status\" <" + s.from + ">\r\n" +
			"To: " + email + "\r\n" +
			"Subject: Endpoint Status Update \r\n" +
			"\r\n",
	))
}
