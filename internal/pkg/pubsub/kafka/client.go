package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

var ErrInvalidCredentials = errors.New("your kafka credentials are invalid. please verify you're providing the correct credentials")

type Kafka struct {
	Cfg     *datastore.KafkaPubSubConfig
	source  *datastore.Source
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Kafka {
	ctx, cancel := context.WithCancel(context.Background())

	return &Kafka{
		Cfg:     source.PubSub.Kafka,
		source:  source,
		workers: source.PubSub.Workers,
		handler: handler,
		ctx:     ctx,
		done:    make(chan struct{}),
		cancel:  cancel,
		log:     log,
	}
}

func (k *Kafka) Start() {
	for i := 1; i <= k.workers; i++ {
		go k.Consume()
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

func (k *Kafka) cancelled() bool {
	select {
	case <-k.done:
		return true
	default:
		return false
	}
}

func (k *Kafka) Verify() error {
	dialer, err := k.dialer()
	if err != nil {
		return err
	}

	_, err = dialer.DialContext(context.Background(), "tcp", k.Cfg.Brokers[0])
	if err != nil {
		log.WithError(err).Error("failed to connect to kafka instance")
		return err
	}

	return nil

}

func (k *Kafka) Consume() {
	dialer, err := k.dialer()
	if err != nil {
		log.WithError(err).Error("failed to fetch kafka auth")
		return
	}

	consumerGroup := k.Cfg.ConsumerGroupID
	if util.IsStringEmpty(consumerGroup) {
		// read from all groups.
		consumerGroup = " "
	}

	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.Cfg.Brokers,
		GroupID: consumerGroup,
		Topic:   k.Cfg.TopicName,
		Dialer:  dialer,
	})

	defer k.handleError(r)

	for {
		if k.cancelled() {
			return
		}

		m, err := r.FetchMessage(k.ctx)
		if err != nil {
			log.WithError(err).Errorf("failed to fetch message from topic %s - kafka", k.Cfg.TopicName)
			continue
		}

		ctx := context.Background()
		if err := k.handler(ctx, k.source, string(m.Value)); err != nil {
			k.log.WithError(err).Error("failed to write message to create event queue - kafka pub sub")
		} else {
			// acknowledge the message
			err := r.CommitMessages(ctx, m)
			if err != nil {
				k.log.WithError(err).Error("failed to commit message - kafka pub sub")
			}
		}
	}
}

func (k *Kafka) Stop() {
	k.cancel()
	close(k.done)
}

func (k *Kafka) handleError(reader *kafka.Reader) {
	if err := reader.Close(); err != nil {
		k.log.WithError(err).Error("an error occurred while closing the kafka client")
	}

	if err := recover(); err != nil {
		k.log.WithError(fmt.Errorf("sourceID: %s, Errror: %s", k.source.UID, err)).Error("kafka pubsub source crashed")
	}
}
