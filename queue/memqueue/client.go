package memqueue

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/disq"
	localstorage "github.com/frain-dev/disq/brokers/localstorage"
)

type MemQueuer struct {
	m              sync.Map
	defaultOptions queue.QueueOptions
}

func NewQueuer(defaultOptions queue.QueueOptions) queue.Queuer {
	return &MemQueuer{
		defaultOptions: defaultOptions,
	}
}

func (q *MemQueuer) NewQueue(opts queue.QueueOptions) error {
	cfg := &localstorage.LocalStorageConfig{
		Name:       opts.Name,
		Concurency: int32(opts.Concurrency),
		BufferSize: convoy.BufferSize,
	}

	b := localstorage.New(cfg)

	_, loaded := q.m.LoadOrStore(b.Name(), b)
	if loaded {
		err := fmt.Errorf("queue with name=%q already exists", b.Name())
		return err
	}
	log.Printf("succesfully added queue=%s", b.Name())
	return nil
}

func (q *MemQueuer) Write(ctx context.Context, taskname string, queuename string, job *queue.Job) error {
	m := &disq.Message{
		Ctx:      ctx,
		TaskName: string(taskname),
		Args:     []interface{}{job},
		Delay:    job.Delay,
	}
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}
	err = b.Publish(m)
	if err != nil {
		return err
	}
	return nil
}

func (q *MemQueuer) StartOne(ctx context.Context, queuename string) error {
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

func (q *MemQueuer) StartAll(ctx context.Context) error {
	q.m.Range(func(key, value interface{}) bool {
		b := value.(disq.Broker)
		if !b.Status() {
			b.Consume(ctx)
		}
		return true
	})
	return nil
}

func (q *MemQueuer) Delete(queuename string) error {
	b, err := q.Load(queuename)
	if err != nil {
		return err
	}
	err = b.Stop()
	if err != nil {
		log.Printf("error stopping queue=%s:%s", queuename, err)
	}
	_, loaded := q.m.LoadAndDelete(queuename)
	if loaded {
		log.Printf("queue with name=%q deleted", queuename)
		return nil
	}
	return nil
}

func (q *MemQueuer) Length(queuename string) (int, error) {
	b, err := q.Load(queuename)
	if err != nil {
		return 0, err
	}
	return b.Len()
}

func (p *MemQueuer) Update(ctx context.Context, opts queue.QueueOptions) error {
	if v, ok := p.m.LoadAndDelete(opts.Name); ok {
		b := v.(disq.Broker)
		err := b.Stop()
		if err != nil {
			return err
		}
		_ = p.NewQueue(opts)
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

func (q *MemQueuer) Stats(queuename string) (*queue.Stats, error) {
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

func (q *MemQueuer) StopOne(name string) error {
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

func (p *MemQueuer) StopAll() error {
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

func (q *MemQueuer) Load(queuename string) (disq.Broker, error) {
	if v, ok := q.m.Load(queuename); ok {
		q := v.(disq.Broker)
		return q, nil
	}
	return nil, fmt.Errorf("queue with name=%q not found", queuename)
}

func (p *MemQueuer) Contains(name string) bool {
	if _, ok := p.m.Load(name); ok {
		return ok
	}
	return false
}
