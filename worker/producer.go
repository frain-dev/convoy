package worker

import (
	"context"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	convoyMemqueue "github.com/frain-dev/convoy/queue/memqueue"
	convoyRedis "github.com/frain-dev/convoy/queue/redis"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Producer struct {
	scheduleQueue queue.Queuer
	consumer      *taskq.Consumer
	quit          chan chan error
}

func NewProducer(cfg config.Configuration, eventQueue queue.Queuer) *Producer {
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
			err := consumer.Stop()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	return &Producer{
		scheduleQueue: queue,
		consumer:      consumer.(*taskq.Consumer),
	}
}

func (p *Producer) Start() {
	go func() {
		err := p.consumer.Start(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func (p *Producer) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
