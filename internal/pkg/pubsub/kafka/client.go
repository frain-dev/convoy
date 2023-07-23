package kafka

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/segmentio/kafka-go"
)

type Kafka struct {
	Cfg     *datastore.KafkaPubSubConfig
	source  *datastore.Source
	workers int
	done    chan struct{}
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Kafka {
	return &Kafka{
		Cfg:     source.PubSub.KafKa,
		source:  source,
		workers: source.PubSub.Workers,
		done:    make(chan struct{}),
		log:     log,
	}
}

func (k *Kafka) Start() {
	for i := 1; i <= k.workers; i++ {
		go k.Consume()
	}
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
	_, err := kafka.DialLeader(context.Background(), "tcp", k.Cfg.Brokers[0], k.Cfg.TopicName, 0)
	if err != nil {
		return err
	}

	return nil

}

func (k *Kafka) Consume() {
	// make a new reader that consumes from topic
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.Cfg.Brokers,
		GroupID:  k.Cfg.ConsumerGroupID,
		Topic:    k.Cfg.TopicName,
		MaxBytes: 10e6, // 10MB
	})

	defer k.handleError(r)

	for {
		if k.cancelled() {
			return
		}

		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.WithError(err).Errorf("failed to read message from topic %s - kafka", k.Cfg.TopicName)
		}

		fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
	}
}

func (k *Kafka) Stop() {
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
