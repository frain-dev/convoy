package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Producer struct {
	Data            chan queue.Message
	msgRepo         *convoy.MessageRepository
	dispatch        *net.Dispatcher
	signatureConfig config.SignatureConfiguration
	quit            chan chan error
}

func NewProducer(queuer *queue.Queuer, msgRepo *convoy.MessageRepository, signatureConfig config.SignatureConfiguration) *Producer {
	return &Producer{
		Data:            (*queuer).Read(),
		msgRepo:         msgRepo,
		dispatch:        net.NewDispatcher(),
		signatureConfig: signatureConfig,
		quit:            make(chan chan error),
	}
}

func (p *Producer) Start() {
	go func() {
		for {
			select {
			case data := <-p.Data:
				go func() {
					p.postMessages(*p.msgRepo, data.Data)
				}()
			case ch := <-p.quit:
				close(p.Data)
				close(ch)
				return
			}
		}
	}()
}

func (p *Producer) postMessages(msgRepo convoy.MessageRepository, m convoy.Message) {

	var attempt convoy.MessageAttempt
	var secret = m.AppMetadata.Secret

	var done = true
	for i := range m.AppMetadata.Endpoints {

		e := &m.AppMetadata.Endpoints[i]
		if e.Sent {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
			done = done && true
			continue
		}

		request := models.WebhookRequest{
			Event: string(m.EventType),
			Data:  m.Data,
		}

		bytes, err := json.Marshal(request)
		if err != nil {
			log.Errorf("error occurred while parsing payload - %+v\n", err)
			return
		}

		bStr := string(bytes)
		hmac, err := util.ComputeJSONHmac(p.signatureConfig.Hash, secret, bStr, false)
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
