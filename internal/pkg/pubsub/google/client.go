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
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Google {
	return &Google{
		Cfg:     source.PubSub.Google,
		source:  source,
		workers: source.PubSub.Workers,
		handler: handler,
		log:     log,
	}
}

func (g *Google) Start(ctx context.Context) {
	g.ctx = ctx
	if g.workers > 0 {
		go g.consume()
	}
}

// Verify ensures the pub sub credentials are valid
func (g *Google) Verify() error {
	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, g.Cfg.ProjectID, option.WithCredentialsJSON(g.Cfg.ServiceAccount))
	if err != nil {
		log.WithError(err).Error("failed to create new pubsub client")
		return ErrInvalidCredentials
	}

	defer g.handleError(client)

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

func (g *Google) consume() {
	client, err := pubsub.NewClient(g.ctx, g.Cfg.ProjectID, option.WithCredentialsJSON(g.Cfg.ServiceAccount))

	if err != nil {
		g.log.WithError(err).Error("failed to create new pubsub client")
	}

	defer g.handleError(client)

	sub := client.Subscription(g.Cfg.SubscriptionID)

	// NumGoroutines determines the number of goroutines sub.Receive will spawn to pull messages
	sub.ReceiveSettings.NumGoroutines = g.workers

	err = sub.Receive(g.ctx, func(ctx context.Context, m *pubsub.Message) {
		if err := g.handler(ctx, g.source, string(m.Data)); err != nil {
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

func (g *Google) handleError(client *pubsub.Client) {
	if err := client.Close(); err != nil {
		g.log.WithError(err).Error("an error occurred while closing the client")
	}

	if err := recover(); err != nil {
		g.log.WithError(fmt.Errorf("sourceID: %s, Errror: %s", g.source.UID, err)).Error("google pubsub source crashed")
	}
}
