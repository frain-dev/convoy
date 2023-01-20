package sqs

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type handlerFn func(source *datastore.Source, queue queue.Queuer, msg string) error

type Sqs struct {
	cfg     *datastore.SQSPubSubConfig
	source  *datastore.Source
	workers int
	done    chan struct{}
	queue   queue.Queuer
	handler handlerFn
}

func New(source *datastore.Source, queue queue.Queuer, handler handlerFn) *Sqs {
	return &Sqs{
		cfg:     source.PubSubConfig.Sqs,
		source:  source,
		workers: source.PubSubConfig.Workers,
		done:    make(chan struct{}),
		queue:   queue,
		handler: handler,
	}
}

func (s *Sqs) Dispatch() {
	for i := 1; i <= s.workers; i++ {
		go s.Listen()
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

func (s *Sqs) Listen() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.cfg.AccessKeyID, s.cfg.SecretKey, ""),
	})

	if err != nil {
		log.WithError(err).Error("failed to create new session - sqs")
	}

	svc := sqs.New(sess)
	url, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.cfg.QueueName,
	})

	if err != nil {
		log.WithError(err).Error("failed to fetch queue url - sqs")
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
			log.WithError(err).Error("failed to fetch message - sqs")
		}

		var wg sync.WaitGroup
		for _, message := range output.Messages {
			wg.Add(1)
			go func(m *sqs.Message) {
				defer wg.Done()

				if err := s.handler(s.source, s.queue, *m.Body); err != nil {
					log.WithError(err).Error("failed to write message to create event queue")
				}

				_, err = svc.DeleteMessage(&sqs.DeleteMessageInput{
					QueueUrl:      queueURL,
					ReceiptHandle: m.ReceiptHandle,
				})

				if err != nil {
					log.WithError(err).Error("failed to delete message")
				}
			}(message)

			wg.Wait()
		}
	}
}

func (s *Sqs) Stop() {
	close(s.done)
}
