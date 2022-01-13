package memqueue

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/vmihailenco/taskq/v3"
	"github.com/vmihailenco/taskq/v3/memqueue"
)

type MemQueue struct {
	Name      string
	queue     *memqueue.Queue
	inner     taskq.Factory
	closeChan chan struct{}
}

type MClient struct{}

func NewQueueClient() queue.QueueClient {
	return &MClient{}
}

func (client *MClient) NewClient(cfg config.Configuration) (*queue.StorageClient, taskq.Factory, error) {
	if cfg.Queue.Type != config.InMemoryQueueProvider {
		return nil, nil, errors.New("please select the in-memory driver in your config")
	}

	qFn := memqueue.NewFactory()

	sc := &queue.StorageClient{
		Memclient: queue.NewLocalStorage(),
	}

	return sc, qFn, nil
}

func (client *MClient) NewQueue(localstorage queue.StorageClient, factory taskq.Factory, name string) queue.Queuer {
	q := factory.RegisterQueue(&taskq.QueueOptions{
		Name:    name,
		Storage: localstorage.Memclient,
	})

	return &MemQueue{
		Name:  name,
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
