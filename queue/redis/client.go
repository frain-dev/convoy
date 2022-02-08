package redis

import (
	"context"
	"errors"
	"math"
	"strings"
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

const count = math.MaxInt64

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

func (q *RedisQueue) ZRangebyScore(ctx context.Context, min string, max string) ([]string, error) {
	zset := q.stringifyZSETWithQName()
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
	stream := q.stringifyStreamWithQName()
	pending, err := q.inner.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  convoy.StreamGroup,
		Start:  start,
		End:    end,
		Count:  count,
	}).Result()
	if err != nil {
		if strings.HasPrefix(err.Error(), "NOGROUP") {
			_ = q.inner.XGroupCreateMkStream(ctx, q.stringifyStreamWithQName(), convoy.StreamGroup, "0").Err()
		}
		return nil, err
	}
	return pending, nil
}

func (q *RedisQueue) XRange(ctx context.Context, start string, end string) *redis.XMessageSliceCmd {
	stream := q.stringifyStreamWithQName()
	xrange := q.inner.XRange(ctx, stream, start, end)
	return xrange
}

func (q *RedisQueue) XRangeN(ctx context.Context, start string, end string, count int64) *redis.XMessageSliceCmd {
	stream := q.stringifyStreamWithQName()
	xrange := q.inner.XRangeN(ctx, stream, start, end, count)
	return xrange
}

func (q *RedisQueue) XPending(ctx context.Context) (*redis.XPending, error) {
	stream := q.stringifyStreamWithQName()
	pending, err := q.inner.XPending(ctx, stream, convoy.StreamGroup).Result()
	if err != nil {
		if strings.HasPrefix(err.Error(), "NOGROUP") {
			_ = q.inner.XGroupCreateMkStream(ctx, q.stringifyStreamWithQName(), convoy.StreamGroup, "0").Err()
		}
	}
	return pending, err
}

func (q *RedisQueue) XInfoConsumers(ctx context.Context) *redis.XInfoConsumersCmd {
	stream := q.stringifyStreamWithQName()
	consumersInfo := q.inner.XInfoConsumers(ctx, stream, convoy.StreamGroup)
	return consumersInfo
}

func (q *RedisQueue) XInfoStream(ctx context.Context) *redis.XInfoStreamCmd {
	stream := q.stringifyStreamWithQName()
	infoStream := q.inner.XInfoStream(ctx, stream)
	return infoStream
}

func (q *RedisQueue) CheckEventDeliveryinStream(ctx context.Context, id string, start string, end string) (bool, error) {
	xmsgs, err := q.XRange(ctx, start, end).Result()
	if err != nil {
		return false, err
	}

	msgs := make([]taskq.Message, len(xmsgs))
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = unmarshalMessage(msg, xmsg)

		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			return true, nil
		}
	}
	return false, nil
}

func (q *RedisQueue) CheckEventDeliveryinZSET(ctx context.Context, id string, min string, max string) (bool, error) {
	bodies, err := q.ZRangebyScore(ctx, min, max)
	if err != nil {
		return false, err
	}
	var msg taskq.Message
	for _, body := range bodies {
		err := msg.UnmarshalBinary([]byte(body))

		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			return true, nil
		}
	}
	return false, nil
}

func (q *RedisQueue) DeleteEventDeliveryFromZSET(ctx context.Context, id string) (bool, error) {
	bodies, err := q.ZRangebyScore(ctx, "-inf", "+inf")
	if err != nil {
		return false, err
	}

	var msg taskq.Message
	for _, body := range bodies {
		err := msg.UnmarshalBinary([]byte(body))
		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			zset := q.stringifyZSETWithQName()
			intCmd := q.inner.ZRem(ctx, zset, body)
			if err = intCmd.Err(); err != nil {
				return false, err
			}
		}
	}
	return false, nil
}

func (q *RedisQueue) DeleteEventDeliveriesFromZSET(ctx context.Context, ids []string) error {
	bodies, err := q.ZRangebyScore(ctx, "-inf", "+inf")
	if err != nil {
		return err
	}

	idMap := map[string]string{}

	var msg taskq.Message
	for _, body := range bodies {
		err := msg.UnmarshalBinary([]byte(body))
		if err != nil {
			return err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		idMap[value] = body
	}

	for _, id := range ids {
		if body, ok := idMap[id]; ok {
			intCmd := q.inner.ZRem(ctx, q.stringifyZSETWithQName(), body)
			if err = intCmd.Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *RedisQueue) CheckEventDeliveryinPending(ctx context.Context, id string) (bool, error) {
	pending, err := q.XPending(ctx)
	if err != nil {
		return false, err
	}
	if pending.Count <= 0 {
		return false, nil
	}
	pendingXmgs, err := q.XRangeN(ctx, pending.Lower, pending.Higher, pending.Count).Result()
	if err != nil {
		return false, err
	}

	msgs := make([]taskq.Message, len(pendingXmgs))

	for i := range pendingXmgs {
		xmsg := &pendingXmgs[i]
		msg := &msgs[i]

		err = unmarshalMessage(msg, xmsg)
		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			return true, nil
		}

	}
	return false, nil
}

func (q *RedisQueue) DeleteEvenDeliveryfromStream(ctx context.Context, id string) (bool, error) {
	xmsgs, err := q.XRange(ctx, "-", "+").Result()
	if err != nil {
		return false, err
	}
	msgs := make([]taskq.Message, len(xmsgs))
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = unmarshalMessage(msg, xmsg)

		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			if err := q.inner.XAck(ctx, q.stringifyStreamWithQName(), convoy.StreamGroup, xmsg.ID).Err(); err != nil {
				return true, err
			}
			return true, q.inner.XDel(ctx, q.stringifyStreamWithQName(), xmsg.ID).Err()
		}
	}
	return false, nil
}

func (q *RedisQueue) DeleteEventDeliveriesFromStream(ctx context.Context, ids []string) error {
	xmsgs, err := q.XRange(ctx, "-", "+").Result()
	if err != nil {
		return err
	}
	msgs := make([]taskq.Message, len(xmsgs))

	idMap := map[string]*redis.XMessage{}
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = unmarshalMessage(msg, xmsg)

		if err != nil {
			return err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		idMap[value] = xmsg

	}

	for _, id := range ids {
		if xmsg, ok := idMap[id]; ok {
			if err := q.inner.XAck(ctx, q.stringifyStreamWithQName(), convoy.StreamGroup, xmsg.ID).Err(); err != nil {
				return err
			}

			err = q.inner.XDel(ctx, q.stringifyStreamWithQName(), xmsg.ID).Err()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *RedisQueue) stringifyStreamWithQName() string {
	return "taskq:" + "{" + q.Name + "}:stream"
}
func (q *RedisQueue) stringifyZSETWithQName() string {
	return "taskq:" + "{" + q.Name + "}:zset"
}

func unmarshalMessage(msg *taskq.Message, xmsg *redis.XMessage) error {
	body := xmsg.Values["body"].(string)
	err := msg.UnmarshalBinary([]byte(body))
	if err != nil {
		return err
	}

	msg.ID = xmsg.ID
	return nil
}
