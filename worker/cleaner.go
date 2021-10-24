package worker

import (
	"github.com/Subomi/taskq/v3"
	"github.com/frain-dev/convoy/queue"
	convoy_redis "github.com/frain-dev/convoy/queue/redis"
	log "github.com/sirupsen/logrus"
)

type Cleaner struct {
	deadLetterQueue queue.Queuer
	consumer        *taskq.Consumer
	quit            chan chan error
}

func NewCleaner(queue *convoy_redis.RedisQueue) *Cleaner {
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

func (p *Cleaner) Close() error {
	ch := make(chan error)
	p.quit <- ch
	return <-ch
}
