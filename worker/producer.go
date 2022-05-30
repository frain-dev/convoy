package worker

import (
	"context"

	"github.com/frain-dev/convoy/queue"
	log "github.com/sirupsen/logrus"
)

type Producer struct {
	queue queue.Queuer
}

func NewProducer(queue queue.Queuer) *Producer {
	return &Producer{
		queue: queue,
	}
}

func (p *Producer) Start(ctx context.Context) {
	err := p.queue.StartAll(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func (p *Producer) Stop() error {
	return p.queue.StopAll()
}

func (p *Producer) Queuer() error {
	return p.Queuer()
}
