package sqs

import (
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

var ErrInvalidCredentials = errors.New("your sqs credentials are invalid. please verify you're providing the correct credentials")

type Sqs struct {
	Cfg     *datastore.SQSPubSubConfig
	source  *datastore.Source
	workers int
	done    chan struct{}
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Sqs {
	return &Sqs{
		Cfg:     source.PubSub.Sqs,
		source:  source,
		workers: source.PubSub.Workers,
		done:    make(chan struct{}),
		handler: handler,
		log:     log,
	}
}

func (s *Sqs) Start() {
	for i := 1; i <= s.workers; i++ {
		go s.Consume()
	}
}

func (s *Sqs) cancelled() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Verify ensures the sqs credentials are correct
func (s *Sqs) Verify() error {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.Cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.Cfg.AccessKeyID, s.Cfg.SecretKey, ""),
	})

	if err != nil {
		log.WithError(err).Error("failed to create new session - sqs")
		return ErrInvalidCredentials
	}

	svc := sqs.New(sess)
	_, err = svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.Cfg.QueueName,
	})

	if err != nil {
		log.WithError(err).Error("failed to fetch queue url - sqs")
		return ErrInvalidCredentials
	}

	return nil
}

func (s *Sqs) Consume() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.Cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.Cfg.AccessKeyID, s.Cfg.SecretKey, ""),
	})

	if err != nil {
		log.WithError(err).Error("failed to create new session - sqs")
	}

	svc := sqs.New(sess)
	url, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.Cfg.QueueName,
	})

	if err != nil {
		s.log.WithError(err).Error("failed to fetch queue url - sqs")
	}

	queueURL := url.QueueUrl

	for {
		if s.cancelled() {
			return
		}

		output, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: aws.Int64(10),
			WaitTimeSeconds:     aws.Int64(1),
		})

		if err != nil {
			s.log.WithError(err).Error("failed to fetch message - sqs")
		}

		var wg sync.WaitGroup
		for _, message := range output.Messages {
			wg.Add(1)
			go func(m *sqs.Message) {
				defer wg.Done()

				if err := s.handler(s.source, *m.Body); err != nil {
					s.log.WithError(err).Error("failed to write message to create event queue")
				} else {
					_, err = svc.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl:      queueURL,
						ReceiptHandle: m.ReceiptHandle,
					})

					if err != nil {
						s.log.WithError(err).Error("failed to delete message")
					}
				}

			}(message)

			wg.Wait()
		}
	}
}

func (s *Sqs) Stop() {
	close(s.done)
}
