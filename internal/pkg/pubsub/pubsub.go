package pubsub

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	rqm "github.com/frain-dev/convoy/internal/pkg/pubsub/amqp"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/kafka"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/pkg/log"
)

type PubSub interface {
	Start(context.Context)
	Consume()
	Stop()
}

type PubSubSource struct {
	cancelFunc context.CancelFunc

	// use channel to trigger a reset.
	ctx context.Context

	// The pub sub client.
	client PubSub

	// The DB source
	source *datastore.Source

	// This is a hash for the source config used to
	// track if an existing source config has been changed.
	hash string
}

func NewPubSubSource(ctx context.Context, source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) (*PubSubSource, error) {
	client, err := NewPubSubClient(source, handler, log)
	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(ctx)
	pubSubSource := &PubSubSource{client: client, source: source}
	pubSubSource.hash = generateSourceKey(source)
	pubSubSource.cancelFunc = cancelFunc
	return pubSubSource, nil
}

func (p *PubSubSource) Start() {
	p.client.Start(p.ctx)
}

func (p *PubSubSource) Stop() {
	p.cancelFunc()
}

func NewPubSubClient(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) (PubSub, error) {
	if source.PubSub.Type == datastore.SqsPubSub {
		return sqs.New(source, handler, log), nil
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		return google.New(source, handler, log), nil
	}

	if source.PubSub.Type == datastore.KafkaPubSub {
		return kafka.New(source, handler, log), nil
	}

	if source.PubSub.Type == datastore.AmqpPubSub {
		return rqm.New(source, handler, log), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", source.PubSub.Type)
}

func generateSourceKey(source *datastore.Source) string {
	var hash string

	if source.PubSub.Type == datastore.SqsPubSub {
		sq := source.PubSub.Sqs
		hash = fmt.Sprintf("%s,%s,%s,%s,%v", sq.AccessKeyID, sq.SecretKey, sq.DefaultRegion, sq.QueueName, source.PubSub.Workers)
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		gq := source.PubSub.Google
		hash = fmt.Sprintf("%s,%s,%s,%v", gq.ServiceAccount, gq.ProjectID, gq.SubscriptionID, source.PubSub.Workers)
	}

	if source.PubSub.Type == datastore.KafkaPubSub {
		kq := source.PubSub.Kafka
		hash = fmt.Sprintf("%s,%s,%s,%v,%v", kq.Brokers, kq.ConsumerGroupID, kq.TopicName, kq.Auth, source.PubSub.Workers)
	}

	if source.PubSub.Type == datastore.AmqpPubSub {
		aq := source.PubSub.Amqp
		hash = fmt.Sprintf("%s,%s,%s,%v", aq.Schema, aq.Host, aq.Queue, source.PubSub.Workers)
	}

	h := md5.Sum([]byte(hash))
	hash = hex.EncodeToString(h[:])

	return hash
}
