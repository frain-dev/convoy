package worker

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/queue"
	convoyRedis "github.com/frain-dev/convoy/queue/redis"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Producer struct {
	scheduleQueue queue.Queuer
	consumer      *taskq.Consumer
	quit          chan chan error
}

func NewProducer(queue *convoyRedis.RedisQueue) *Producer {
	consumer := queue.Consumer()

	return &Producer{
		scheduleQueue: queue,
		consumer:      consumer.(*taskq.Consumer),
	}
}

func (p *Producer) Start() {
	ctx := context.Background()
	go func() {
		err := p.consumer.Start(ctx)
		if err != nil {
			log.Fatal(err)
		}

		ticker := time.NewTicker(2000 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				log.Printf("Consumer Stats: %+v\n", p.consumer.Stats())
			case <-ctx.Done():
				log.Println("Consumer quiting")
				return
			}
		}
	}()
}

func (p *Producer) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
