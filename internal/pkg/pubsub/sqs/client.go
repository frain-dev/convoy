package sqs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/msgpack"

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
	Cfg         *datastore.SQSPubSubConfig
	source      *datastore.Source
	workers     int
	ctx         context.Context
	handler     datastore.PubSubHandler
	log         log.StdLogger
	rateLimiter limiter.RateLimiter
	licenser    license.Licenser
	instanceId  string
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger, rateLimiter limiter.RateLimiter, licenser license.Licenser, instanceId string) *Sqs {
	return &Sqs{
		Cfg:         source.PubSub.Sqs,
		source:      source,
		workers:     source.PubSub.Workers,
		handler:     handler,
		log:         log,
		rateLimiter: rateLimiter,
		licenser:    licenser,
		instanceId:  instanceId,
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

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Errorf("failed to load config.Get() in sqs source %s with id %s", s.source.Name, s.source.UID)
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if !util.IsStringEmpty(s.instanceId) {
				err = s.rateLimiter.Allow(s.ctx, s.instanceId, cfg.InstanceIngestRate)
				if err != nil {
					time.Sleep(time.Millisecond * 250)
					continue
				}
			}

			allAttr := "All"
			output, err := svc.ReceiveMessageWithContext(s.ctx, &sqs.ReceiveMessageInput{
				QueueUrl:              queueURL,
				WaitTimeSeconds:       aws.Int64(1),
				MessageAttributeNames: []*string{&allAttr},
			})
			if err != nil {
				s.log.WithError(err).Error("failed to fetch message - sqs")
				continue
			}

			mm := metrics.GetDPInstance(s.licenser)
			mm.IncrementIngestTotal(s.source.UID, s.source.ProjectID)

			var wg sync.WaitGroup
			for _, message := range output.Messages {
				wg.Add(1)
				go func(m *sqs.Message) {
					defer wg.Done()

					defer s.handleError()

					var d Attrs = m.MessageAttributes

					attributes, err := msgpack.EncodeMsgPack(d.Map())
					if err != nil {
						s.log.WithError(err).Error("failed to marshall message attributes")
						return
					}

					// Google Pub/Sub sends a slice with a single non UTF-8 value,
					// looks like this: [192], which can cause a panic when marshaling headers
					if len(attributes) == 1 && attributes[0] == 192 {
						emptyMap := map[string]string{}
						emptyBytes, err := msgpack.EncodeMsgPack(emptyMap)
						if err != nil {
							s.log.WithError(err).Error("an error occurred creating an empty attributes map")
							return
						}
						attributes = emptyBytes
					}

					if err := s.handler(context.Background(), s.source, *m.Body, attributes); err != nil {
						s.log.WithError(err).Error("failed to write message to create event queue")
						mm.IncrementIngestErrorsTotal(s.source)
					} else {
						mm.IncrementIngestConsumedTotal(s.source)
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

type M map[string]any

// Attrs is a representation of Sqs MessageAttributes.
type Attrs map[string]*sqs.MessageAttributeValue

// Map creates a map from the elements of the Attrs.
func (d Attrs) Map() M {
	m := make(M, len(d))
	for k, e := range d {
		m[k] = *e.StringValue
	}
	return m
}
