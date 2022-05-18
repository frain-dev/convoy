package memqueue

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/disq"
	localstorage "github.com/frain-dev/disq/brokers/localstorage"
)

type MemQueue struct {
	Name  string
	queue *localstorage.LocalStorage
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	cfg := &localstorage.LocalStorageConfig{
		Name:       opts.Name,
		BufferSize: convoy.BufferSize,
	}
	q := localstorage.New(cfg)

	return &MemQueue{
		Name:  opts.Name,
		queue: q.((*localstorage.LocalStorage)),
	}
}

func (q *MemQueue) Stop() error {
	return q.queue.Stop()
}

func (q *MemQueue) Publish(ctx context.Context, name convoy.TaskName, job *queue.Job, delay time.Duration) error {

	m := &disq.Message{
		Ctx:      ctx,
		TaskName: string(name),
		Args:     []interface{}{job},
		Delay:    delay,
	}

	err := q.queue.Publish(m)
	if err != nil {
		return err
	}

	return nil
}

func (q *MemQueue) Consume(ctx context.Context) error {
	q.queue.Consume(ctx)
	return nil
}

func (q *MemQueue) Length() (int, error) {
	return q.queue.Len()
}

func (q *MemQueue) Broker() disq.Broker {
	return q.queue
}
