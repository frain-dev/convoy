package task

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

func ProcessEvent(eventRepo convoy.EventRepository, eventDeliveryQueue queue.Queuer) func(*queue.Job) error {
	return func(job *queue.Job) error {
		return nil
	}
}
