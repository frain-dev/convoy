package pubsub

import (
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
)

type PubSub interface {
	Dispatch()
	Listen()
	Stop()
}

func NewPubSub(cfg *datastore.PubSubConfig) (PubSub, error) {
	if cfg.Type == datastore.SqsPubSub {
		return sqs.New(cfg.Sqs), nil
	}

	if cfg.Type == datastore.GooglePubSub {
		return google.New(cfg.Google), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", cfg.Type)
}
