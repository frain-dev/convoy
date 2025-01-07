package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/ingest"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type Kafka struct {
	Cfg         *datastore.KafkaPubSubConfig
	source      *datastore.Source
	workers     int
	ctx         context.Context
	handler     datastore.PubSubHandler
	log         log.StdLogger
	rateLimiter limiter.RateLimiter
	licenser    license.Licenser
	instanceId  string
	ingestCfg   *ingest.IngestCfg
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger, rateLimiter limiter.RateLimiter, licenser license.Licenser, instanceId string, ingestCfg *ingest.IngestCfg) *Kafka {
	return &Kafka{
		Cfg:         source.PubSub.Kafka,
		source:      source,
		workers:     source.PubSub.Workers,
		handler:     handler,
		log:         log,
		rateLimiter: rateLimiter,
		licenser:    licenser,
		instanceId:  instanceId,
		ingestCfg:   ingestCfg,
	}
}

func (k *Kafka) Start(ctx context.Context) {
	k.ctx = ctx

	for i := 1; i <= k.workers; i++ {
		go k.consume()
	}
}

func (k *Kafka) dialer() (*kafka.Dialer, error) {
	var mechanism sasl.Mechanism
	var err error

	dialer := &kafka.Dialer{
		Timeout:   15 * time.Second,
		DualStack: true,
	}

	auth := k.Cfg.Auth
	if auth != nil {
		if auth.Type != "plain" && auth.Type != "scram" {
			return nil, fmt.Errorf("auth type: %s is not supported", auth.Type)
		}

		if auth.Type == "plain" {
			mechanism = plain.Mechanism{
				Username: auth.Username,
				Password: auth.Password,
			}
		}

		if auth.Type == "scram" {
			algo := scram.SHA512

			if auth.Hash == "SHA256" {
				algo = scram.SHA256
			}

			mechanism, err = scram.Mechanism(algo, auth.Username, auth.Password)
			if err != nil {
				return nil, err
			}
		}

		dialer.SASLMechanism = mechanism

		if auth.TLS {
			dialer.TLS = &tls.Config{}
		}
	}

	return dialer, nil
}

func (k *Kafka) Verify() error {
	dialer, err := k.dialer()
	if err != nil {
		return err
	}

	_, err = dialer.DialContext(context.Background(), "tcp", k.Cfg.Brokers[0])
	if err != nil {
		return err
	}

	return nil
}

func (k *Kafka) consume() {
	dialer, err := k.dialer()
	if err != nil {
		log.WithError(err).Errorf("failed to fetch auth for kafka source %s with id %s", k.source.Name, k.source.UID)
		return
	}

	consumerGroup := k.Cfg.ConsumerGroupID
	if util.IsStringEmpty(consumerGroup) {
		// read from all groups.
		consumerGroup = " "
	}

	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:               k.Cfg.Brokers,
		GroupID:               consumerGroup,
		Topic:                 k.Cfg.TopicName,
		Dialer:                dialer,
		WatchPartitionChanges: true,
	})

	defer k.handleError(r)

	for {
		select {
		case <-k.ctx.Done():
			return
		default:
			if !util.IsStringEmpty(k.instanceId) {
				instanceIngestRate, err := k.ingestCfg.GetInstanceRateLimitWithCache(k.ctx)
				if err != nil {
					log.WithError(err).Errorf("failed to determine ingest rate from kafka source %s with id %s from topic %s - kafka", k.source.Name, k.source.UID, k.Cfg.TopicName)
					continue
				}
				err = k.rateLimiter.Allow(k.ctx, k.instanceId, instanceIngestRate)
				if err != nil {
					time.Sleep(time.Millisecond * 250)
					continue
				}
			}

			m, err := r.FetchMessage(k.ctx)
			if err != nil {
				log.WithError(err).Errorf("failed to fetch message from kafka source %s with id %s from topic %s - kafka", k.source.Name, k.source.UID, k.Cfg.TopicName)
				continue
			}

			mm := metrics.GetDPInstance(k.licenser)
			mm.IncrementIngestTotal(k.source.UID, k.source.ProjectID)

			var d D = m.Headers
			headers, err := msgpack.EncodeMsgPack(d.Map())
			if err != nil {
				k.log.WithError(err).Error("failed to marshall message headers")
			}

			if err := k.handler(k.ctx, k.source, string(m.Value), headers); err != nil {
				k.log.WithError(err).Errorf("failed to write message from kafka source %s with id %s to create event queue - kafka pub sub", k.source.Name, k.source.UID)
				mm.IncrementIngestErrorsTotal(k.source)
			} else {
				// acknowledge the message
				err := r.CommitMessages(k.ctx, m)
				if err != nil {
					k.log.WithError(err).Error("failed to commit message - kafka pub sub")
					mm.IncrementIngestErrorsTotal(k.source)
				} else {
					mm.IncrementIngestConsumedTotal(k.source)
				}
			}
		}
	}
}

func (k *Kafka) handleError(reader *kafka.Reader) {
	if err := reader.Close(); err != nil {
		k.log.WithError(err).Error("an error occurred while closing the kafka client")
	}

	if err := recover(); err != nil {
		k.log.WithError(fmt.Errorf("sourceID: %s, Error: %s", k.source.UID, err)).Error("kafka pubsub source crashed")
	}
}

type M map[string]any

// D is an array representation of Kafka Headers.
type D []kafka.Header

// Map creates a map from the elements of the D.
func (d D) Map() M {
	m := make(M, len(d))
	for _, e := range d {
		m[e.Key] = e.Value
	}
	return m
}
