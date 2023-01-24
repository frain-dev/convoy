package google

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"google.golang.org/api/option"
)

type Google struct {
	cfg     *datastore.GooglePubSubConfig
	source  *datastore.Source
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	handler datastore.PubSubHandler
}

func New(source *datastore.Source, handler datastore.PubSubHandler) *Google {
	ctx, cancel := context.WithCancel(context.Background())

	return &Google{
		cfg:     source.PubSubConfig.Google,
		source:  source,
		ctx:     ctx,
		cancel:  cancel,
		workers: source.PubSubConfig.Workers,
		handler: handler,
	}
}

func (g *Google) Start() {
	go g.Consume()
}

func (g *Google) Stop() {
	g.cancel()
}

func (g *Google) Consume() {
	client, err := pubsub.NewClient(context.Background(), g.cfg.ProjectID, option.WithCredentialsJSON(g.cfg.Credentials))

	if err != nil {
		log.WithError(err).Error("failed to create new pubsub client")
	}

	defer client.Close()

	sub := client.Subscription(g.cfg.SubscriptionID)

	// To enable concurrency settings
	sub.ReceiveSettings.Synchronous = false
	// NumGoroutines determines the number of goroutines sub.Receive will spawn to pull messages
	sub.ReceiveSettings.NumGoroutines = g.workers
	// MaxOutstandingMessages limits the number of concurrent handlers of messages
	sub.ReceiveSettings.MaxOutstandingMessages = 8

	err = sub.Receive(g.ctx, func(ctx context.Context, m *pubsub.Message) {
		if err := g.handler(g.source, string(m.Data)); err != nil {
			log.WithError(err).Error("failed to write message to create event queue - google pub sub")
		}

		m.Ack()
	})

	if err != nil {
		log.WithError(err).Error("sub receive error")
		return
	}
}
