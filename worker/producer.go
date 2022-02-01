package worker

import (
	"context"

	"github.com/frain-dev/convoy/queue"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Producer struct {
	scheduleQueue queue.Queuer
	consumer      *taskq.Consumer
	quit          chan chan error
}

func NewProducer(queue queue.Queuer) *Producer {
	consumer := queue.Consumer()

	return &Producer{
		scheduleQueue: queue,
		consumer:      consumer.(*taskq.Consumer),
	}
}

func (p *Producer) Start(ctx context.Context) {
	go func() {
		err := p.consumer.Start(ctx)
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
