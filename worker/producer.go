package worker

import (
	"context"

	"github.com/frain-dev/convoy/queue"
	convoy_redis "github.com/frain-dev/convoy/queue/redis"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Producer struct {
	scheduleQueue queue.Queuer
	consumer      *taskq.Consumer
	quit          chan chan error
}

func NewProducer(queue *convoy_redis.RedisQueue) *Producer {
	consumer := queue.Consumer()

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
