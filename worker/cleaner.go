package worker

import (
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/worker/otel"
	"github.com/frain-dev/taskq/v3"
	log "github.com/sirupsen/logrus"
)

type Cleaner struct {
	deadLetterQueue queue.Queuer
	consumer        *taskq.Consumer
	quit            chan chan error
}

func NewCleaner(queue queue.Queuer) *Cleaner {

	consumer := queue.Consumer()

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

func (p *Cleaner) Otel(tr tracer.Tracer) {
	otelHook := otel.NewOtelHook(tr)
	p.consumer.AddHook(otelHook)
}

func (p *Cleaner) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
