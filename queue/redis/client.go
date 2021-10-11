package redis

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/taskq/v3"
	"github.com/vmihailenco/taskq/v3/redisq"
)

type RedisQueue struct {
	Name      string
	queue     *redisq.Queue
	inner     *redis.Client
	closeChan chan struct{}
}

func NewClient(cfg config.Configuration) (*redis.Client, taskq.Factory, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, nil, errors.New("please select the redis driver in your config")
	}

	dsn := cfg.Queue.Redis.DSN
	if util.IsStringEmpty(dsn) {
		return nil, nil, errors.New("please provide the Redis DSN")
	}

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, nil, err
	}

	c := redis.NewClient(opts)
	if err := c.
		Ping(context.Background()).
		Err(); err != nil {
		return nil, nil, err
	}

	qFn := redisq.NewFactory()

	return c, qFn, nil
}

func NewQueue(c *redis.Client, factory taskq.Factory, name string) queue.Queuer {

	q := factory.RegisterQueue(&taskq.QueueOptions{
		Name:  name,
		Redis: c,
	})

	return &RedisQueue{
		Name:  name,
		inner: c,
		queue: q.(*redisq.Queue),
	}
}

func (c *RedisQueue) Close() error {
	c.closeChan <- struct{}{}
	return c.inner.Close()
}

func (c *RedisQueue) Write(ctx context.Context, msg *convoy.Message, delay time.Duration) error {
	job := &queue.Job{
		Data: msg,
	}
	m := &taskq.Message{
		Ctx:      ctx,
		TaskName: c.Name,
		Args:     []interface{}{ctx, *job},
		Delay:    delay,
	}

	err := c.queue.Add(m)
	if err != nil {
		return err
	}

	return nil
}

func (c *RedisQueue) Inner() *redisq.Queue {
	return c.queue
}
