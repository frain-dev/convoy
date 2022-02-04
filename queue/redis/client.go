package redis

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
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

func NewQueue(opts queue.QueueOptions) queue.Queuer {

	q := opts.Factory.RegisterQueue(&taskq.QueueOptions{
		Name:  opts.Name,
		Redis: opts.Redis,
	})

	return &RedisQueue{
		Name:  opts.Name,
		inner: opts.Redis,
		queue: q.(*redisq.Queue),
	}
}

func (q *RedisQueue) Close() error {
	q.closeChan <- struct{}{}
	return q.inner.Close()
}

func (q *RedisQueue) Write(ctx context.Context, name convoy.TaskName, e *datastore.EventDelivery, delay time.Duration) error {
	job := &queue.Job{
		ID: e.UID,
	}

	m := &taskq.Message{
		Ctx:      ctx,
		TaskName: string(name),
		Args:     []interface{}{job},
		Delay:    delay,
	}

	return q.queue.Add(m)
}

func (q *RedisQueue) Consumer() taskq.QueueConsumer {
	return q.queue.Consumer()
}
