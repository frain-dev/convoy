package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/smtp"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Producer struct {
	Data            chan queue.Message
	appRepo         *convoy.ApplicationRepository
	msgRepo         *convoy.MessageRepository
	dispatch        *net.Dispatcher
	signatureConfig config.SignatureConfiguration
	smtpConfig      config.SMTPConfiguration
	quit            chan chan error
}

func NewProducer(queuer *queue.Queuer, appRepo *convoy.ApplicationRepository, msgRepo *convoy.MessageRepository, signatureConfig config.SignatureConfiguration, smtpConfig config.SMTPConfiguration) *Producer {
	return &Producer{
		Data:            (*queuer).Read(),
		appRepo:         appRepo,
		msgRepo:         msgRepo,
		dispatch:        net.NewDispatcher(),
		signatureConfig: signatureConfig,
		smtpConfig:      smtpConfig,
		quit:            make(chan chan error),
	}
}

func (p *Producer) Start() {
	go func() {
		for {
			select {
			case data := <-p.Data:
				go func() {
					p.postMessages(*p.msgRepo, *p.appRepo, data.Data)
				}()
			case ch := <-p.quit:
				close(p.Data)
				close(ch)
				return
			}
		}
	}()
}

func (p *Producer) postMessages(msgRepo convoy.MessageRepository, appRepo convoy.ApplicationRepository, m convoy.Message) {

	var attempt convoy.MessageAttempt
	var secret = m.AppMetadata.Secret

	var done = true
	for i := range m.AppMetadata.Endpoints {

		e := &m.AppMetadata.Endpoints[i]
		if e.Sent {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
			continue
		}

		bytes, err := json.Marshal(m.Data)
		if err != nil {
			log.Errorf("error occurred while parsing json")
			return
		}

		bStr := string(bytes)
		hmac, err := util.ComputeJSONHmac(p.signatureConfig.Hash, bStr, secret, false)
		if err != nil {
			log.Errorf("error occurred while generating hmac signature - %+v\n", err)
			return
		}

		attemptStatus := convoy.FailureMessageStatus
		start := time.Now()

		resp, err := p.dispatch.SendRequest(e.TargetURL, string(convoy.HttpPost), bytes, string(p.signatureConfig.Header), hmac)
		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(start)
		// log request details
		requestLogger := log.WithFields(log.Fields{
			"status":   status,
			"uri":      e.TargetURL,
			"method":   convoy.HttpPost,
			"duration": duration,
		})

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			requestLogger.Infof("%s", m.UID)
			log.Infof("%s sent\n", m.UID)
			attemptStatus = convoy.SuccessMessageStatus
			e.Sent = true
		} else {
			requestLogger.Errorf("%s", m.UID)
			done = false
			e.Sent = false
		}
		if err != nil {
			log.Errorf("%s failed. Reason: %s", m.UID, err)
		}

		attempt = parseAttemptFromResponse(m, *e, resp, attemptStatus)
	}
	m.Metadata.NumTrials++
	if done {
		m.Status = convoy.SuccessMessageStatus
		m.Description = ""

		// TODO: If endpoint is disabled.
		// Enable it both in the Endpoint and in the EndpointMetadata.

	} else {
		m.Status = convoy.RetryMessageStatus

		delay := m.Metadata.IntervalSeconds
		nextTime := time.Now().Add(time.Duration(delay) * time.Second)
		m.Metadata.NextSendTime = primitive.NewDateTimeFromTime(nextTime)

		log.Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", m.UID, nextTime.Format(time.ANSIC), m.Metadata.Strategy, delay, m.Metadata.NumTrials, m.Metadata.RetryLimit)
	}

	if m.Metadata.NumTrials >= m.Metadata.RetryLimit {
		if done {
			if m.Status != convoy.SuccessMessageStatus {
				log.Errorln("an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
				m.Status = convoy.FailureMessageStatus
			}
		} else {
			log.Errorf("%s retry limit exceeded ", m.UID)
			m.Description = "Retry limit exceeded"
			m.Status = convoy.FailureMessageStatus
		}

		go func() {
			inactiveEndpoints := make([]string, 0)
			for i := 0; i < len(m.AppMetadata.Endpoints); i++ {
				endpoint := m.AppMetadata.Endpoints[i]
				if !endpoint.Sent {
					inactiveEndpoints = append(inactiveEndpoints, endpoint.UID)
				}
			}
			err := appRepo.UpdateApplicationEndpointsAsDisabled(context.Background(), m.AppID, inactiveEndpoints, true)
			if err != nil {
				log.WithError(err).Error("Failed to update disabled app endpoints")
				return
			}

			s, err := smtp.New(&p.smtpConfig)
			if err == nil {
				for i := 0; i < len(m.AppMetadata.Endpoints); i++ {
					email := m.AppMetadata.SupportEmail
					endpoint := m.AppMetadata.Endpoints[i]
					err = s.SendEmailNotification(email, endpoint)
					if err != nil {
						log.WithError(err).Error("Failed to send notification email")
					}
				}
			}
		}()
	}

	err := msgRepo.UpdateMessageWithAttempt(context.Background(), m, attempt)
	if err != nil {
		log.Errorln("failed to update message ", m.UID)
	}
}

func parseAttemptFromResponse(m convoy.Message, e convoy.EndpointMetadata, resp *net.Response, attemptStatus convoy.MessageStatus) convoy.MessageAttempt {

	return convoy.MessageAttempt{
		ID:         primitive.NewObjectID(),
		UID:        uuid.New().String(),
		MsgID:      m.UID,
		EndpointID: e.UID,
		APIVersion: "2021-08-27",

		IPAddress:        resp.IP,
		Header:           resp.Header,
		ContentType:      resp.ContentType,
		HttpResponseCode: resp.Status,
		ResponseData:     string(resp.Body),
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
}

func (p *Producer) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
