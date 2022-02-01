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
		Name:         opts.Name,
		Redis:        opts.Redis,
		MaxNumWorker: 100,
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

	err := q.queue.Add(m)
	if err != nil {
		return err
	}

	return nil
}

func (q *RedisQueue) Consumer() taskq.QueueConsumer {
	return q.queue.Consumer()
}

func (q *RedisQueue) ZRangebyScore(ctx context.Context, min string, max string) ([]string, error) {
	zset := "taskq:" + "{" + q.Name + "}:zset"
	bodies, err := q.inner.ZRangeByScore(ctx, zset, &redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
	if err != nil {
		return nil, err
	}
	return bodies, nil
}

func (q *RedisQueue) XPendingExt(ctx context.Context, start string, end string) ([]redis.XPendingExt, error) {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	streamGroup := "taskq"
	pending, err := q.inner.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  streamGroup,
		Start:  start,
		End:    end,
	}).Result()
	if err != nil {
		return nil, err
	}
	return pending, redis.Nil
}

func (q *RedisQueue) XRange(ctx context.Context, start string, end string) *redis.XMessageSliceCmd {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	xrange := q.inner.XRange(ctx, stream, start, end)
	return xrange
}

func (q *RedisQueue) XRangeN(ctx context.Context, start string, end string, count int64) *redis.XMessageSliceCmd {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	xrange := q.inner.XRangeN(ctx, stream, start, end, count)
	return xrange
}

func (q *RedisQueue) XPending(ctx context.Context) *redis.XPendingCmd {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	streamGroup := "taskq"
	pending := q.inner.XPending(ctx, stream, streamGroup)
	return pending
}

func (q *RedisQueue) XInfoConsumers(ctx context.Context) *redis.XInfoConsumersCmd {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	streamGroup := "taskq"
	consumersInfo := q.inner.XInfoConsumers(ctx, stream, streamGroup)
	return consumersInfo
}

func (q *RedisQueue) XInfoStream(ctx context.Context) *redis.XInfoStreamCmd {
	stream := "taskq:" + "{" + q.Name + "}:stream"
	infoStream := q.inner.XInfoStream(ctx, stream)
	return infoStream
}
