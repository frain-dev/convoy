package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/go-redis/redis/v8"
)

const (
	defaultChannel = "convoy"
)

type client struct {
	inner         *redis.Client
	pubsubChannel *redis.PubSub
	closeChan     chan struct{}
}

func New(cfg config.Configuration) (queue.Queuer, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	dsn := cfg.Queue.Redis.DSN
	if util.IsStringEmpty(dsn) {
		return nil, errors.New("please provide the Redis DSN")
	}

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, err
	}

	c := redis.NewClient(opts)
	if err := c.
		Ping(context.Background()).
		Err(); err != nil {
		return nil, err
	}

	pubsubCh := c.Subscribe(context.Background(), defaultChannel)

	return &client{
		inner:         c,
		pubsubChannel: pubsubCh,
	}, nil
}

func (c *client) Close() error {
	c.closeChan <- struct{}{}
	return c.inner.Close()
}

func (c *client) Read() chan queue.Message {
	channels := make(chan queue.Message)

	go func() {
		for msg := range c.pubsubChannel.Channel() {
			var m convoy.Message

			if err := json.NewDecoder(strings.
				NewReader(msg.Payload)).
				Decode(&m); err != nil {

				channels <- queue.Message{
					Err: err,
				}
				continue
			}

			channels <- queue.Message{
				Err:  nil,
				Data: m,
			}
		}
	}()

	return channels
}

func (c *client) Write(ctx context.Context,
	msg convoy.Message) error {
	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(&msg); err != nil {
		return err
	}

	return c.inner.Publish(ctx, defaultChannel, b.Bytes()).Err()
}
