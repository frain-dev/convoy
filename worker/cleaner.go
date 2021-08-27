package worker

import (
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/queue"
	"github.com/hookcamp/hookcamp/worker/task"
	log "github.com/sirupsen/logrus"
)

type Cleaner struct {
	queue   *queue.Queuer
	msgRepo *hookcamp.MessageRepository
}

func NewCleaner(queuer *queue.Queuer, msgRepo *hookcamp.MessageRepository) *Cleaner {
	return &Cleaner{
		queue:   queuer,
		msgRepo: msgRepo,
	}
}

func (c *Cleaner) Start() {
	go func() {
		log.Debugln("Running cleanup tasks")
		task.RetryAbandonedMessages(*c.queue, *c.msgRepo)
	}()
}
