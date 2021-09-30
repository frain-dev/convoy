package smtp

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

const (
	NotificationSubject  = "Endpoint Status Update"
	NotificationTemplate = "endpoint.update.html"
)

type SmtpClient struct {
	url, username, password, from string
	port                          uint32
}

func New(cfg *config.SMTPConfiguration) (*SmtpClient, error) {
	var err error

	errMsg := "Missing SMTP Config - %s"
	if util.IsStringEmpty(cfg.URL) {
		err = fmt.Errorf(errMsg, "URL")
		log.WithError(err).Error()
	}

	if cfg.Port == 0 {
		err = fmt.Errorf(errMsg, "Port")
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
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}, err
}

func (s *SmtpClient) SendEmailNotification(email string, endpoint convoy.EndpointMetadata) error {
	// Compose Message
	m := s.setHeaders(email)

	// Set body
	templ, err := template.ParseFiles(NotificationTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse notification template")
	}

	var body bytes.Buffer
	err = templ.Execute(&body, struct {
		URL    string
		Status convoy.EndpointStatus
	}{
		URL:    endpoint.TargetURL,
		Status: endpoint.Status,
	})

	if err != nil {
		log.WithError(err).Error("Failed to build template")
		return err
	}
	m.SetBody("text/html", body.String())

	d := gomail.NewDialer(s.url, int(s.port), s.username, s.password)
	// Send Email
	if err = d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func (s *SmtpClient) setHeaders(email string) *gomail.Message {
	m := gomail.NewMessage()

	m.SetHeader("From", fmt.Sprintf("Convoy Status <%s>", s.from))
	m.SetHeader("To", email)
	m.SetHeader("Subject", NotificationSubject)

	return m
}
