package smtp

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

//go:embed endpoint.update.html
var t string

const (
	NotificationSubject  = "Endpoint Status Update"
	NotificationTemplate = "endpoint.update.html"
)

type SmtpClient struct {
	url, username, password, from, replyTo string
	port                                   uint32
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
		replyTo:  cfg.ReplyTo,
	}, err
}

func (s *SmtpClient) SendEmailNotification(email, logoURL, targetURL string, status datastore.EndpointStatus) error {
	// Compose Message
	m := s.setHeaders(email)

	// Parse Template
	templ := template.Must(template.New("notificationEmail").Parse(t))

	// Set data.
	var body bytes.Buffer
	err := templ.Execute(&body, struct {
		URL     string
		LogoURL string
		Status  datastore.EndpointStatus
	}{
		URL:     targetURL,
		LogoURL: logoURL,
		Status:  status,
	})

	if err != nil {
		log.WithError(err).Error("Failed to build template")
		return err
	}
	m.SetBody("text/html", body.String())

	// Send Email
	d := gomail.NewDialer(s.url, int(s.port), s.username, s.password)
	if err = d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func (s *SmtpClient) setHeaders(email string) *gomail.Message {
	m := gomail.NewMessage()

	m.SetHeader("From", fmt.Sprintf("Convoy Status <%s>", s.from))
	m.SetHeader("To", email)

	if !util.IsStringEmpty(s.replyTo) {
		m.SetHeader("Reply-To", s.replyTo)
	}

	m.SetHeader("Subject", NotificationSubject)

	return m
}
