package redis

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/disq"
	redisBroker "github.com/frain-dev/disq/brokers/redis"
	"github.com/go-redis/redis/v8"
)

type FIFOQueue struct {
	Name      string
	queue     *redisBroker.List
	inner     *redis.Client
	closeChan chan struct{}
}

func NewClient(cfg config.Configuration) (*redis.Client, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	dsn := cfg.Queue.Redis.Dsn
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

	return c, nil
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {

	cfg := &redisBroker.RedisConfig{
		Name:            opts.Name,
		Redis:           opts.Redis,
		ReservationSize: convoy.ReservationSize,
		BufferSize:      convoy.BufferSize,
	}
	q := redisBroker.NewList(cfg)

	return &FIFOQueue{
		Name:  opts.Name,
		inner: opts.Redis,
		queue: q.(*redisBroker.List),
	}
}

func (q *FIFOQueue) Stop() error {
	q.closeChan <- struct{}{}
	err := q.inner.Close()
	if err != nil {
		return err
	}
	err = q.queue.Stop()
	if err != nil {
		return err
	}
	return nil
}

func (q *FIFOQueue) Publish(ctx context.Context, name convoy.TaskName, job *queue.Job, delay time.Duration) error {
	m := &disq.Message{
		Ctx:      ctx,
		TaskName: string(name),
		Args:     []interface{}{job},
		Delay:    delay,
	}

	return q.queue.Publish(m)
}

func (q *FIFOQueue) Consume(ctx context.Context) error {
	q.queue.Consume(ctx)
	return nil
}

func (q *FIFOQueue) Length() (int, error) {
	return q.queue.Len()
}

func (q *FIFOQueue) Broker() disq.Broker {
	return q.queue
}
