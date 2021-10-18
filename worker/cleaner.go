package worker

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	log "github.com/sirupsen/logrus"
)

type Cleaner struct {
	queue   *queue.Queuer
	msgRepo *convoy.MessageRepository
}

func NewCleaner(queuer *queue.Queuer, msgRepo *convoy.MessageRepository) *Cleaner {
	return &Cleaner{
		queue:   queuer,
		msgRepo: msgRepo,
	}
}

func (c *Cleaner) Start() {
	go func() {
		log.Infoln("Running cleanup tasks")
		task.RetryAbandonedMessages(*c.queue, *c.msgRepo)
	}()
}
