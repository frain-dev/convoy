package sqs

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/pkg/log"
)

type SQS struct {
	accessKeyID   string
	secretKey     string
	defaultRegion string
	queueName     string
	workers       int
	done          chan struct{}
}

func newSqsPubSub(cfg *datastore.SQSPubSubConfig) pubsub.PubSub {
	return &SQS{
		accessKeyID:   cfg.AccessKeyID,
		secretKey:     cfg.SecretKey,
		defaultRegion: cfg.DefaultRegion,
		queueName:     cfg.QueueName,
		done:          make(chan struct{}),
	}
}

func (s *SQS) Dispatch() {
	for i := 1; i <= s.workers; i++ {
		go s.Listen()
	}
}

func (s *SQS) cancelled() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *SQS) Listen() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.defaultRegion),
		Credentials: credentials.NewStaticCredentials(s.accessKeyID, s.secretKey, ""),
	})

	if err != nil {
		log.WithError(err).Error("failed to create new session - sqs")
	}

	svc := sqs.New(sess)
	url, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.queueName,
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

				svc.DeleteMessage(&sqs.DeleteMessageInput{
					QueueUrl:      queueURL,
					ReceiptHandle: m.ReceiptHandle,
				})
			}(message)

			wg.Wait()
		}
	}
}

func (s *SQS) Stop() {
	close(s.done)
}
