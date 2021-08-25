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
				p.postMessages(*p.msgRepo, data.Data)
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
	for i := range m.Application.Endpoints {

		e := &m.Application.Endpoints[i]
		if e.Merged != nil && *e.Merged {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
			continue
		}

		attemptStatus := hookcamp.FailureMessageStatus

		resp, err := p.dispatch.SendRequest(e.TargetURL, string(hookcamp.HttpPost), m.Data)
		if err == nil && resp.Response != nil && resp.Response.Status == "200 OK" {
			log.Debugln("message sent to ", e.TargetURL)
			attemptStatus = hookcamp.SuccessMessageStatus
			if e.Merged != nil {
				*e.Merged = true
			} else {
				e.Merged = newBool(true)
			}
		} else {
			log.Debugln("failed to send messages to ", e.TargetURL)
			done = false
			if e.Merged != nil {
				*e.Merged = false
			} else {
				e.Merged = newBool(false)
			}
		}
		if err != nil {
			log.Debugln("failed to create message attempt")
		}

		attempt := parseAttemptFromResponse(m, *e, resp, attemptStatus)
		m.MessageAttempts = append(m.MessageAttempts, attempt)
	}
	if done {
		m.Status = hookcamp.SuccessMessageStatus
	} else {
		m.Status = hookcamp.RetryMessageStatus
		m.Metadata.NextSendTime = time.Now().Add(15 * time.Second).Unix() // TODO: define strategy for retrials
	}
	m.Metadata.NumTrials += 1

	if m.Metadata.NumTrials >= m.Metadata.RetryLimit {
		log.Errorln("retry limit exceeded for message - ", m.UID)
		m.Description = "retry limit exceeded"
		m.Status = hookcamp.FailureMessageStatus
	}

	err := msgRepo.UpdateMessage(context.Background(), m)
	if err != nil {
		log.Errorln("failed to update message ", m.UID)
	}
}

func newBool(b bool) *bool {
	return &b
}

func parseAttemptFromResponse(m hookcamp.Message, e hookcamp.Endpoint, resp *net.Response, attemptStatus hookcamp.MessageStatus) hookcamp.MessageAttempt {
	ip := ""
	userAgent := ""
	httpStatus := ""
	body := make([]byte, 0)
	if resp != nil {
		r := resp.Response
		if r != nil {
			httpStatus = r.Status
		}
		ip = resp.IP
		userAgent = resp.UserAgent
		body = resp.Body
	}

	return hookcamp.MessageAttempt{
		ID:         primitive.NewObjectID(),
		UID:        uuid.New().String(),
		MsgID:      m.UID,
		EndpointID: e.UID,
		APIVersion: "2021-08-27",

		IPAddress:        ip,
		UserAgent:        userAgent,
		HttpResponseCode: httpStatus,
		ResponseData:     string(body),
		Status:           attemptStatus,

		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
}

func (p *Producer) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
