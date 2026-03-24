package sqs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	common "github.com/frain-dev/convoy/internal/pkg/pubsub/const"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/util"
)

var ErrInvalidCredentials = errors.New("your sqs credentials are invalid. please verify you're providing the correct credentials")

type Sqs struct {
	Cfg         *datastore.SQSPubSubConfig
	source      *datastore.Source
	workers     int
	ctx         context.Context
	handler     datastore.PubSubHandler
	log         log.Logger
	rateLimiter limiter.RateLimiter
	licenser    license.Licenser
	instanceId  string
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.Logger, rateLimiter limiter.RateLimiter, licenser license.Licenser, instanceId string) *Sqs {
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
	awsCfg := &aws.Config{
		Region:      aws.String(s.Cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.Cfg.AccessKeyID, s.Cfg.SecretKey, ""),
	}

	// Support custom endpoint for LocalStack testing
	if s.Cfg.Endpoint != "" {
		awsCfg.Endpoint = aws.String(s.Cfg.Endpoint)
		awsCfg.DisableSSL = aws.Bool(true)
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		slog.Error("failed to create new session - sqs", "error", err)
		return ErrInvalidCredentials
	}

	svc := sqs.New(sess)
	_, err = svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.Cfg.QueueName,
	})

	if err != nil {
		slog.Error("failed to fetch queue url - sqs", "error", err)
		return ErrInvalidCredentials
	}

	return nil
}

func (s *Sqs) consume() {
	awsCfg := &aws.Config{
		Region:      aws.String(s.Cfg.DefaultRegion),
		Credentials: credentials.NewStaticCredentials(s.Cfg.AccessKeyID, s.Cfg.SecretKey, ""),
	}

	// Support custom endpoint for LocalStack testing
	if s.Cfg.Endpoint != "" {
		awsCfg.Endpoint = aws.String(s.Cfg.Endpoint)
		awsCfg.DisableSSL = aws.Bool(true)
	}

	sess, err := session.NewSession(awsCfg)

	defer s.handleError()

	if err != nil {
		s.log.Error("failed to create new session - sqs", "error", err)
	}

	svc := sqs.New(sess)
	url, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.Cfg.QueueName,
	})
	if err != nil {
		s.log.Error("failed to fetch queue url - sqs", "error", err)
	}

	if url == nil {
		slog.Error(fmt.Sprintf("pubsub url for source with id %s is nil", s.source.UID))
		return
	}

	if url.QueueUrl == nil {
		slog.Error(fmt.Sprintf("pubsub queue url for source with id %s is nil", s.source.UID))
		return
	}

	if util.IsStringEmpty(*url.QueueUrl) {
		slog.Error(fmt.Sprintf("pubsub queue url for source with id %s is empty", s.source.UID))
		return
	}

	queueURL := url.QueueUrl

	cfg, err := config.Get()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to load config.Get() in sqs source %s with id %s: %v", s.source.Name, s.source.UID, err))
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
				s.log.Error("failed to fetch message - sqs", "error", err)
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

					if d == nil {
						d = Attrs{}
					}

					// Add message_id to attributes
					d[common.BrokerMessageHeader] = &sqs.MessageAttributeValue{StringValue: m.MessageId, DataType: aws.String("String")}

					attributes, err := msgpack.EncodeMsgPack(d.Map())
					if err != nil {
						s.log.Error("failed to marshall message attributes", "error", err)
						return
					}

					// Google Pub/Sub sends a slice with a single non UTF-8 value,
					// looks like this: [192], which can cause a panic when marshaling headers
					if len(attributes) == 1 && attributes[0] == 192 {
						emptyMap := map[string]string{}
						emptyBytes, err := msgpack.EncodeMsgPack(emptyMap)
						if err != nil {
							s.log.Error("an error occurred creating an empty attributes map", "error", err)
							return
						}
						attributes = emptyBytes
					}

					if err := s.handler(context.Background(), s.source, *m.Body, attributes); err != nil {
						s.log.Error("failed to write message to create event queue", "error", err)
						mm.IncrementIngestErrorsTotal(s.source)
					} else {
						mm.IncrementIngestConsumedTotal(s.source)
						_, err = svc.DeleteMessage(&sqs.DeleteMessageInput{
							QueueUrl:      queueURL,
							ReceiptHandle: m.ReceiptHandle,
						})

						if err != nil {
							s.log.Error("failed to delete message", "error", err)
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
		s.log.Error("sqs pubsub source crashed", "error", fmt.Errorf("sourceID: %s, Error: %s", s.source.UID, err))
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
