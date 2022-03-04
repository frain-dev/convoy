package memqueue

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/taskq/v3"
	"github.com/frain-dev/taskq/v3/memqueue"
)

type MemQueue struct {
	Name      string
	queue     *memqueue.Queue
	inner     taskq.Factory
	closeChan chan struct{}
}

func NewClient(cfg config.Configuration) (queue.Storage, taskq.Factory, error) {
	if cfg.Queue.Type != config.InMemoryQueueProvider {
		return nil, nil, errors.New("please select the in-memory queue in your config")
	}

	qFn := memqueue.NewFactory()

	storage := queue.NewLocalStorage()

	return storage, qFn, nil
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	q := opts.Factory.RegisterQueue(&taskq.QueueOptions{
		Name:            opts.Name,
		Storage:         opts.Storage,
		MaxNumFetcher:   convoy.MaxNumFetcher,
		ReservationSize: convoy.ReservationSize,
		BufferSize:      convoy.BufferSize,
	})

	return &MemQueue{
		Name:  opts.Name,
		inner: opts.Factory,
		queue: q.(*memqueue.Queue),
	}
}

func (q *MemQueue) Close() error {
	q.closeChan <- struct{}{}
	return q.inner.Close()
}

func (q *MemQueue) Write(ctx context.Context, name convoy.TaskName, e *datastore.EventDelivery, delay time.Duration) error {
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

func (q *MemQueue) Consumer() taskq.QueueConsumer {
	return q.queue.Consumer()
}

func (q *MemQueue) Length() (int, error) {
	return q.queue.Len()
}
