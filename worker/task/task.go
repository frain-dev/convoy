package task

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

var (
	EventProcessor = taskq.TaskOptions{
		Name:       "EventProcessor",
		RetryLimit: 10,
		Handler:    PostMessages,
	}
)

func PostMessages(job *queue.Job) {

}

func RetryMessages(queue queue.Queuer, msgRepo convoy.MessageRepository) {
	m, err := msgRepo.LoadMessagesForPostingRetry(context.Background())
	if err != nil {
		log.Errorln("failed to load messages to retry - ", err)
		return
	}

	log.Debugln("loaded retry messages with size: ", len(m))

	err = msgRepo.UpdateStatusOfMessages(context.Background(), m, convoy.ProcessingMessageStatus)
	if err != nil {
		log.Errorln("failed to update status of messages - ", err)
	}
	queueMessages(queue, m)
}

func RetryAbandonedMessages(queue queue.Queuer, msgRepo convoy.MessageRepository) {
	m, err := msgRepo.LoadAbandonedMessagesForPostingRetry(context.Background())
	if err != nil {
		log.Errorln("failed to load abandoned messages to retry - ", err)
		return
	}

	log.Debugln("loaded abandoned messages with size: ", len(m))

	queueMessages(queue, m)
}

func queueMessages(q queue.Queuer, messages []convoy.Message) {
	for _, m := range messages {
		err := q.Write(context.Background(), m)
		if err != nil {
			log.Errorln("failed to write message to queue - ", err)
			return
		}
	}
}
