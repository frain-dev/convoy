package worker

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	convoyMemqueue "github.com/frain-dev/convoy/queue/memqueue"
	convoyRedis "github.com/frain-dev/convoy/queue/redis"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Cleaner struct {
	deadLetterQueue queue.Queuer
	consumer        *taskq.Consumer
	quit            chan chan error
}

func NewCleaner(cfg config.Configuration, eventQueue queue.Queuer) *Cleaner {
	var consumer taskq.QueueConsumer
	var queue queue.Queuer

	if cfg.Queue.Type == config.RedisQueueProvider {
		if queue, ok := eventQueue.(*convoyRedis.RedisQueue); ok {
			consumer = queue.Consumer()
		}
	}

	if cfg.Queue.Type == config.InMemoryQueueProvider {
		if queue, ok := eventQueue.(*convoyMemqueue.MemQueue); ok {
			consumer = queue.Consumer()
			consumer.Stop()
		}
	}

	return &Cleaner{
		deadLetterQueue: queue,
		consumer:        consumer.(*taskq.Consumer),
	}
}

func (c *Cleaner) Start() {
	go func() {
		log.Debugln("Running cleanup tasks")
	}()
}

func (p *Cleaner) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
