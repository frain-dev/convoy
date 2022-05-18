package worker

import (
	"context"

	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/disq"
	log "github.com/sirupsen/logrus"
)

type Producer struct {
	Queues []queue.Queuer
	worker *disq.Worker
}

func NewProducer(queues []queue.Queuer) *Producer {
	brokers := make([]disq.Broker, len(queues))
	for i, q := range queues {
		brokers[i] = q.Broker()
	}
	w := disq.NewWorker(brokers)
	return &Producer{
		Queues: queues,
		worker: w,
	}
}

func (p *Producer) Start(ctx context.Context) {
	go func() {
		err := p.worker.Start(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func (p *Producer) Stop() error {
	return p.worker.Stop()
}
