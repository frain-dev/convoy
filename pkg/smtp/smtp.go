package smtp

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

type SmtpClient interface {
	SendEmail(emailAddr, subject string, body bytes.Buffer) error
}

type Client struct {
	url, username, password, from, replyTo string
	port                                   uint32
}

func New(cfg *config.SMTPConfiguration) (SmtpClient, error) {
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

	return &Client{
		url:      cfg.URL,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		replyTo:  cfg.ReplyTo,
	}, err
}

func (s *Client) SendEmail(emailAddr, subject string, body bytes.Buffer) error {
	// Compose Message
	m := s.setHeaders(emailAddr, subject)

	m.SetBody("text/html", body.String())

	// Send Email
	d := gomail.NewDialer(s.url, int(s.port), s.username, s.password)
	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func (s *Client) setHeaders(email, subject string) *gomail.Message {
	m := gomail.NewMessage()

	m.SetHeader("From", fmt.Sprintf("Convoy Status <%s>", s.from))
	m.SetHeader("To", email)

	if !util.IsStringEmpty(s.replyTo) {
		m.SetHeader("Reply-To", s.replyTo)
	}

	m.SetHeader("Subject", subject)

	return m
}
