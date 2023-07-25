package kafka

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"time"
)

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

func (k *Kafka) auth() (sasl.Mechanism, error) {
	var mechanism sasl.Mechanism
	var err error

	auth := k.Cfg.Auth
	if auth == nil {
		return nil, nil
	}

	if auth.Type == "plain" {
		mechanism = plain.Mechanism{
			Username: auth.Username,
			Password: auth.Password,
		}

		return mechanism, nil
	}

	if auth.Type == "scram" {
		mechanism, err = scram.Mechanism(scram.SHA512, auth.Username, auth.Password)
		if err != nil {
			return nil, err
		}

		return mechanism, nil
	}

	return nil, fmt.Errorf("auth type: %s is not supported", auth.Type)
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
	auth, err := k.auth()
	if err != nil {
		return err
	}

	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: auth,
	}

	_, err = dialer.DialContext(context.Background(), "tcp", k.Cfg.Brokers[0])
	if err != nil {
		return err
	}

	return nil

}

func (k *Kafka) Consume() {
	auth, err := k.auth()
	if err != nil {
		log.WithError(err).Error("failed to fetch kafka auth")
		return
	}

	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.Cfg.Brokers,
		GroupID: k.Cfg.ConsumerGroupID,
		Topic:   k.Cfg.TopicName,
		Dialer: &kafka.Dialer{
			Timeout:       15 * time.Second,
			DualStack:     true,
			SASLMechanism: auth,
		},
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
