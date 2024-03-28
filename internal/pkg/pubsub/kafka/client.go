package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/msgpack"
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
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Kafka {

	return &Kafka{
		Cfg:     source.PubSub.Kafka,
		source:  source,
		workers: source.PubSub.Workers,
		handler: handler,
		log:     log,
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
		Brokers: k.Cfg.Brokers,
		GroupID: consumerGroup,
		Topic:   k.Cfg.TopicName,
		Dialer:  dialer,
	})

	defer k.handleError(r)

	for {
		select {
		case <-k.ctx.Done():
			return
		default:
			m, err := r.FetchMessage(k.ctx)
			if err != nil {
				log.WithError(err).Errorf("failed to fetch message from kafka source %s with id %s from topic %s - kafka", k.source.Name, k.source.UID, k.Cfg.TopicName)
				continue
			}

			var d D = m.Headers

			ctx := context.Background()
			headers, err := msgpack.EncodeMsgPack(d.Map())
			if err != nil {
				k.log.WithError(err).Error("failed to marshall message headers")
			}

			if err := k.handler(ctx, k.source, string(m.Value), headers); err != nil {
				k.log.WithError(err).Errorf("failed to write message from kafka source %s with id %s to create event queue - kafka pub sub", k.source.Name, k.source.UID)
			} else {
				// acknowledge the message
				err := r.CommitMessages(ctx, m)
				if err != nil {
					k.log.WithError(err).Error("failed to commit message - kafka pub sub")
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
//
// Example usage:
//
//	D{{"foo", "bar"}, {"hello", "world"}, {"pi", 3.14159}}
type D []kafka.Header

// Map creates a map from the elements of the D.
func (d D) Map() M {
	m := make(M, len(d))
	for _, e := range d {
		m[e.Key] = e.Value
	}
	return m
}
