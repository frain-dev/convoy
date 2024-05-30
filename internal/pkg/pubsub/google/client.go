package google

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"cloud.google.com/go/pubsub"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"google.golang.org/api/option"
)

var ErrInvalidCredentials = errors.New("your google pub/sub credentials are invalid. please verify you're providing the correct credentials")

type Google struct {
	Cfg         *datastore.GooglePubSubConfig
	source      *datastore.Source
	workers     int
	ctx         context.Context
	handler     datastore.PubSubHandler
	log         log.StdLogger
	rateLimiter limiter.RateLimiter
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger, rateLimiter limiter.RateLimiter) *Google {
	return &Google{
		Cfg:         source.PubSub.Google,
		source:      source,
		workers:     source.PubSub.Workers,
		handler:     handler,
		log:         log,
		rateLimiter: rateLimiter,
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
		attributes, err := msgpack.EncodeMsgPack(m.Attributes)
		if err != nil {
			g.log.WithError(err).Error("failed to marshall message attributes")
			return
		}

		// Google Pub/Sub sends a slice with a single non UTF-8 value,
		// looks like this: [192], which can cause a panic when marshaling headers
		if len(attributes) == 1 && attributes[0] == 192 {
			emptyMap := map[string]string{}
			emptyBytes, err := msgpack.EncodeMsgPack(emptyMap)
			if err != nil {
				g.log.WithError(err).Error("an error occurred creating an empty attributes map")
				return
			}
			attributes = emptyBytes
		}

		mm := metrics.GetDPInstance()
		mm.IncrementIngestTotal(g.source)

		if err := g.handler(ctx, g.source, string(m.Data), attributes); err != nil {
			g.log.WithError(err).Error("failed to write message to create event queue - google pub sub")
			mm.IncrementIngestErrorsTotal(g.source)
		} else {
			m.Ack()
			mm.IncrementIngestConsumedTotal(g.source)
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
		g.log.WithError(fmt.Errorf("sourceID: %s, Error: %s", g.source.UID, err)).Error("google pubsub source crashed")
	}
}
