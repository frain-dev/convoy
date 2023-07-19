package kafka

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

type Kafka struct {
	Cfg     *datastore.KafkaPubSubConfig
	source  *datastore.Source
	workers int
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Kafka {
	return &Kafka{
		Cfg:     source.PubSub.KafKa,
		source:  source,
		workers: source.PubSub.Workers,
		log:     log,
	}
}

func (k *Kafka) Start() {

}

func (k *Kafka) Stop() {

}

func (k *Kafka) Verify() error {
	_, err := kafka.DialLeader(context.Background(), "tcp", k.Cfg.Brokers[0], k.Cfg.TopicName, 0)
	if err != nil {
		return err
	}

	return nil

}

func (k *Kafka) Consume() {

}
