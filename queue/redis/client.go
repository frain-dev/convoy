package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/disq"
	redisBroker "github.com/frain-dev/disq/brokers/redis"
	"github.com/go-redis/redis/v8"
)

type RedisQueuer struct {
	m              sync.Map
	defaultOptions queue.QueueOptions
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

func NewQueuer(defaultOptions queue.QueueOptions) queue.Queuer {
	return &RedisQueuer{
		defaultOptions: defaultOptions}
}

func (q *RedisQueuer) NewQueue(opts queue.QueueOptions) error {

	if opts.Redis == nil {
		opts.Redis = q.defaultOptions.Redis
	}
	cfg := &redisBroker.RedisConfig{
		Name:            opts.Name,
		Redis:           opts.Redis,
		Concurency:      int32(opts.Concurrency),
		ReservationSize: convoy.ReservationSize,
		BufferSize:      convoy.BufferSize,
	}

	b := redisBroker.NewStream(cfg)

	_, loaded := q.m.LoadOrStore(b.Name(), b)
	if loaded {
		err := fmt.Errorf("queue with name=%q already exists", b.Name())
		return err
	}
	log.Printf("succesfully added queue=%s", b.Name())
	return nil
}

func (q *RedisQueuer) Write(ctx context.Context, taskname string, queuename string, job *queue.Job) error {
	var b disq.Broker
	var err error

	m := &disq.Message{
		Ctx:      ctx,
		TaskName: string(taskname),
		Args:     []interface{}{job},
		Delay:    job.Delay,
	}
	b, err = q.Load(queuename)
	if err != nil {
		b, _ = q.Load(q.defaultOptions.Name)
	}
	err = b.Publish(m)
	if err != nil {
		return err
	}
	return nil
}

func (q *RedisQueuer) StartOne(ctx context.Context, queuename string) error {
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}
	if !b.Status() {
		b.Consume(ctx)
		log.Printf("succesfully started queue=%s", queuename)
	}
	return nil
}

func (q *RedisQueuer) StartAll(ctx context.Context) error {
	q.m.Range(func(key, value interface{}) bool {
		b := value.(disq.Broker)
		if !b.Status() {
			b.Consume(ctx)
			log.Printf("succesfully started queue=%s", key)
		}
		return true
	})
	return nil
}

func (q *RedisQueuer) Delete(queuename string) error {
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}
	if b.Status() {
		err = b.Stop()
		if err != nil {
			log.Printf("error stopping queue=%s:%s", queuename, err)
		}
	}
	_, loaded := q.m.LoadAndDelete(queuename)
	if loaded {
		log.Printf("queue with name=%q deleted", queuename)
		return nil
	}
	return nil
}

func (q *RedisQueuer) Length(queuename string) (int, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return 0, err
	}
	return b.Len()
}

func (q *RedisQueuer) Stats(queuename string) (*queue.Stats, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return nil, err
	}
	stats := &queue.Stats{
		Name:      b.Stats().Name,
		Processed: int(b.Stats().Processed),
		Retries:   int(b.Stats().Retries),
		Fails:     int(b.Stats().Fails),
	}
	return stats, nil
}

func (p *RedisQueuer) Update(ctx context.Context, opts queue.QueueOptions) error {
	if v, ok := p.m.LoadAndDelete(opts.Name); ok {
		b := v.(disq.Broker)
		if b.Status() {
			err := b.Stop()
			if err != nil {
				return err
			}
		}
		err := p.NewQueue(opts)
		if err != nil {
			return err
		}
		log.Printf("succesfully updated queue=%s", opts.Name)
		err = p.StartOne(ctx, string(opts.Name))
		if err != nil {
			return err
		}
	} else {
		log.Printf("queue with name=%s not found, adding instead.", opts.Name)
		err := p.NewQueue(opts)
		if err != nil {
			return err
		}
		err = p.StartOne(ctx, string(opts.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (q *RedisQueuer) StopOne(name string) error {
	if v, ok := q.m.Load(name); ok {
		b := v.(disq.Broker)
		if b.Status() {
			err := b.Stop()
			if err != nil {
				return fmt.Errorf("error stopping queue=%s:%s", name, err)
			} else {
				log.Printf("succesfully stopped queue=%s", name)
			}
		}
		return nil
	}
	return fmt.Errorf("queue with name=%q not found", name)
}

func (p *RedisQueuer) StopAll() error {
	p.m.Range(func(key, value interface{}) bool {
		b := value.(disq.Broker)
		if b.Status() {
			err := b.Stop()
			if err != nil {
				log.Printf("error stopping queue=%s:%s", key, err)
			} else {
				log.Printf("succesfully stopped queue=%s", key)
			}
		}
		return true
	})
	return nil
}

func (q *RedisQueuer) Load(queuename string) (disq.Broker, error) {
	if v, ok := q.m.Load(queuename); ok {
		q := v.(disq.Broker)
		return q, nil
	}
	return nil, fmt.Errorf("queue with name=%q not found", queuename)
}

func (p *RedisQueuer) Contains(name string) bool {
	if _, ok := p.m.Load(name); ok {
		return ok
	}
	return false
}

func (q *RedisQueuer) CheckEventDeliveryinStream(ctx context.Context, queuename string, id string, start string, end string) (bool, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return false, err
	}
	xmsgs, err := b.(*redisBroker.Stream).XRange(ctx, start, end).Result()
	if err != nil {
		return false, err
	}

	msgs := make([]disq.Message, len(xmsgs))
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = redisBroker.StreamUnmarshalMessage(msg, xmsg)

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

func (q *RedisQueuer) CheckEventDeliveryinZSET(ctx context.Context, queuename string, id string, min string, max string) (bool, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return false, err
	}

	bodies, err := b.(*redisBroker.Stream).ZRangebyScore(ctx, min, max)
	if err != nil {
		return false, err
	}
	var msg disq.Message
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

func (q *RedisQueuer) DeleteEventDeliveryFromZSET(ctx context.Context, queuename string, id string) (bool, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return false, err
	}

	bodies, err := b.(*redisBroker.Stream).ZRangebyScore(ctx, "-inf", "+inf")
	if err != nil {
		return false, err
	}

	var msg disq.Message
	for _, body := range bodies {
		err := msg.UnmarshalBinary([]byte(body))
		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			intCmd := b.(*redisBroker.Stream).ZRem(ctx, body)
			if err = intCmd.Err(); err != nil {
				return false, err
			}
		}
	}
	return false, nil
}

func (q *RedisQueuer) DeleteEventDeliveriesFromZSET(ctx context.Context, queuename string, ids []string) error {
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}

	bodies, err := b.(*redisBroker.Stream).ZRangebyScore(ctx, "-inf", "+inf")
	if err != nil {
		return err
	}

	idMap := map[string]string{}

	var msg disq.Message
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
			intCmd := b.(*redisBroker.Stream).ZRem(ctx, body)
			if err = intCmd.Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *RedisQueuer) CheckEventDeliveryinPending(ctx context.Context, queuename string, id string) (bool, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return false, err
	}

	pending, err := b.(*redisBroker.Stream).XPending(ctx)
	if err != nil {
		return false, err
	}
	if pending.Count <= 0 {
		return false, nil
	}
	pendingXmgs, err := b.(*redisBroker.Stream).XRangeN(ctx, pending.Lower, pending.Higher, pending.Count).Result()
	if err != nil {
		return false, err
	}

	msgs := make([]disq.Message, len(pendingXmgs))

	for i := range pendingXmgs {
		xmsg := &pendingXmgs[i]
		msg := &msgs[i]

		err = redisBroker.StreamUnmarshalMessage(msg, xmsg)
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

func (q *RedisQueuer) DeleteEvenDeliveryfromStream(ctx context.Context, queuename string, id string) (bool, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return false, err
	}

	xmsgs, err := b.(*redisBroker.Stream).XRange(ctx, "-", "+").Result()
	if err != nil {
		return false, err
	}
	msgs := make([]disq.Message, len(xmsgs))
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = redisBroker.StreamUnmarshalMessage(msg, xmsg)

		if err != nil {
			return false, err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		if value == id {
			if err := b.(*redisBroker.Stream).XAck(ctx, xmsg.ID).Err(); err != nil {
				return true, err
			}
			return true, b.(*redisBroker.Stream).XDel(ctx, xmsg.ID).Err()
		}
	}
	return false, nil
}

func (q *RedisQueuer) DeleteEventDeliveriesFromStream(ctx context.Context, queuename string, ids []string) error {
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}

	xmsgs, err := b.(*redisBroker.Stream).XRange(ctx, "-", "+").Result()
	if err != nil {
		return err
	}
	msgs := make([]disq.Message, len(xmsgs))

	idMap := map[string]*redis.XMessage{}
	for i := range xmsgs {
		xmsg := &xmsgs[i]
		msg := &msgs[i]

		err = redisBroker.StreamUnmarshalMessage(msg, xmsg)

		if err != nil {
			return err
		}

		value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
		idMap[value] = xmsg

	}

	for _, id := range ids {
		if xmsg, ok := idMap[id]; ok {
			if err := b.(*redisBroker.Stream).XAck(ctx, xmsg.ID).Err(); err != nil {
				return err
			}

			err = b.(*redisBroker.Stream).XDel(ctx, xmsg.ID).Err()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
