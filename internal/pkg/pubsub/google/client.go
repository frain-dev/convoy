package google

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"google.golang.org/api/option"
)

var ErrInvalidCredentials = errors.New("your google pub/sub credentials are invalid. please verify you're providing the correct credentials")

type Google struct {
	Cfg     *datastore.GooglePubSubConfig
	source  *datastore.Source
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Google {
	ctx, cancel := context.WithCancel(context.Background())

	return &Google{
		Cfg:     source.PubSub.Google,
		source:  source,
		ctx:     ctx,
		cancel:  cancel,
		workers: source.PubSub.Workers,
		handler: handler,
		log:     log,
	}
}

func (g *Google) Start() {
	go g.Consume()
}

func (g *Google) Stop() {
	g.cancel()
}

// Verify ensures the pub sub credentials are valid
func (g *Google) Verify() error {
	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, g.Cfg.ProjectID, option.WithCredentialsJSON(g.Cfg.ServiceAccount))
	if err != nil {
		log.WithError(err).Error("failed to create new pubsub client")
		return ErrInvalidCredentials
	}

	defer client.Close()

	exists, err := client.Subscription(g.Cfg.SubscriptionID).Exists(ctx)
	if err != nil {
		log.WithError(err).Error("failed to find subscription")
		return ErrInvalidCredentials
	}

	if !exists {
		return fmt.Errorf("subscription ID with name %s does not exist", g.Cfg.SubscriptionID)
	}

	return nil
}

func (g *Google) Consume() {
	client, err := pubsub.NewClient(context.Background(), g.Cfg.ProjectID, option.WithCredentialsJSON(g.Cfg.ServiceAccount))

	if err != nil {
		g.log.WithError(err).Error("failed to create new pubsub client")
	}

	defer client.Close()
	defer g.handleError()

	sub := client.Subscription(g.Cfg.SubscriptionID)

	// To enable concurrency settings
	sub.ReceiveSettings.Synchronous = false
	// NumGoroutines determines the number of goroutines sub.Receive will spawn to pull messages
	sub.ReceiveSettings.NumGoroutines = g.workers

	err = sub.Receive(g.ctx, func(ctx context.Context, m *pubsub.Message) {
		if err := g.handler(g.source, string(m.Data)); err != nil {
			g.log.WithError(err).Error("failed to write message to create event queue - google pub sub")
		} else {
			m.Ack()
		}
	})

	if err != nil {
		g.log.WithError(err).Error("subscription receive error - google pub sub")
		return
	}
}

func (g *Google) handleError() {
	if err := recover(); err != nil {
		g.log.WithError(fmt.Errorf("sourceID: %s, Errror: %s", g.source.UID, err)).Error("googlw pubsub source crashed")
	}
}
