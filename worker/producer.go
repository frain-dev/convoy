package worker

import (
	"context"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/net"
	"github.com/hookcamp/hookcamp/queue"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Producer struct {
	Data     chan queue.Message
	msgRepo  *hookcamp.MessageRepository
	dispatch *net.Dispatcher
	quit     chan chan error
}

func NewProducer(queuer *queue.Queuer, msgRepo *hookcamp.MessageRepository) *Producer {
	return &Producer{
		Data:     (*queuer).Read(),
		msgRepo:  msgRepo,
		dispatch: net.NewDispatcher(),
		quit:     make(chan chan error),
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

func (p *Producer) postMessages(msgRepo hookcamp.MessageRepository, m hookcamp.Message) {

	var done = true
	for i := range m.AppMetadata.Endpoints {

		e := &m.AppMetadata.Endpoints[i]
		if e.Sent {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
			continue
		}

		attemptStatus := hookcamp.FailureMessageStatus
		start := time.Now()

		resp, err := p.dispatch.SendRequest(e.TargetURL, string(hookcamp.HttpPost), m.Data)
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
			"method":   hookcamp.HttpPost,
			"duration": duration,
		})

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			requestLogger.Infof("%s", m.UID)
			log.Infof("%s sent\n", m.UID)
			attemptStatus = hookcamp.SuccessMessageStatus
			e.Sent = true
		} else {
			requestLogger.Errorf("%s", m.UID)
			done = false
			e.Sent = false
		}
		if err != nil {
			log.Errorf("%s failed. Reason: %s", m.UID, err)
		}

		attempt := parseAttemptFromResponse(m, *e, resp, attemptStatus)
		m.MessageAttempts = append([]hookcamp.MessageAttempt{attempt}, m.MessageAttempts...)
	}
	if done {
		m.Status = hookcamp.SuccessMessageStatus
	} else {
		m.Status = hookcamp.RetryMessageStatus
		m.Metadata.NumTrials++

		delay := m.Metadata.IntervalSeconds
		nextTime := time.Now().Add(time.Duration(delay) * time.Second)
		m.Metadata.NextSendTime = primitive.NewDateTimeFromTime(nextTime)

		log.Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", m.UID, nextTime.Format(time.ANSIC), m.Metadata.Strategy, delay, m.Metadata.NumTrials, m.Metadata.RetryLimit)
	}

	if m.Metadata.NumTrials >= m.Metadata.RetryLimit {
		log.Errorf("%s retry limit exceeded ", m.UID)
		m.Description = "Retry limit exceeded"
		m.Status = hookcamp.FailureMessageStatus
	}

	err := msgRepo.UpdateMessage(context.Background(), m)
	if err != nil {
		log.Errorln("failed to update message ", m.UID)
	}
}

func parseAttemptFromResponse(m hookcamp.Message, e hookcamp.EndpointMetadata, resp *net.Response, attemptStatus hookcamp.MessageStatus) hookcamp.MessageAttempt {

	return hookcamp.MessageAttempt{
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
