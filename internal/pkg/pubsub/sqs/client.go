package sqs

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"sync"

	"github.com/frain-dev/convoy/util"

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
	ctx     context.Context
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Sqs {
	return &Sqs{
		Cfg:     source.PubSub.Sqs,
		source:  source,
		workers: source.PubSub.Workers,
		handler: handler,
		log:     log,
	}
}

func (s *Sqs) Start(ctx context.Context) {
	s.ctx = ctx

	for i := 1; i <= s.workers; i++ {
		go s.consume()
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

func (s *Sqs) consume() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.Cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.Cfg.AccessKeyID, s.Cfg.SecretKey, ""),
	})

	defer s.handleError()

	if err != nil {
		s.log.WithError(err).Error("failed to create new session - sqs")
	}

	svc := sqs.New(sess)
	url, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.Cfg.QueueName,
	})

	if err != nil {
		s.log.WithError(err).Error("failed to fetch queue url - sqs")
	}

	if url == nil {
		log.Errorf("pubsub url for source with id %s is nil", s.source.UID)
		log.Errorf("url: %+v\n", url)
		return
	}

	if url.QueueUrl == nil {
		log.Errorf("pubsub queue url for source with id %s is nil", s.source.UID)
		log.Errorf("url: %+v\n", url)
		return
	}

	if util.IsStringEmpty(*url.QueueUrl) {
		log.Errorf("pubsub queue url for source with id %s is empty", s.source.UID)
		log.Errorf("url: %+v\n", url)
		return
	}

	queueURL := url.QueueUrl

	for {

		select {
		case <-s.ctx.Done():
			return
		default:

			output, err := svc.ReceiveMessageWithContext(s.ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            queueURL,
				MaxNumberOfMessages: aws.Int64(10),
				WaitTimeSeconds:     aws.Int64(1),
			})

			if err != nil {
				s.log.WithError(err).Error("failed to fetch message - sqs")
				continue
			}

			var wg sync.WaitGroup
			for _, message := range output.Messages {
				wg.Add(1)
				go func(m *sqs.Message) {
					defer wg.Done()

					defer s.handleError()

					headers, err := msgpack.EncodeMsgPack(m.Attributes)
					if err != nil {
						s.log.WithError(err).Error("failed to marshall message headers")
					}

					if err := s.handler(context.Background(), s.source, *m.Body, headers); err != nil {
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
}

func (s *Sqs) handleError() {
	if err := recover(); err != nil {
		s.log.WithError(fmt.Errorf("sourceID: %s, Error: %s", s.source.UID, err)).Error("sqs pubsub source crashed")
	}
}
