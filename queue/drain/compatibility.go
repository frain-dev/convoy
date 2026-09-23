package drain

import (
	"context"
	"errors"
	"slices"

	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/inventory"
)

type Participant struct {
	StoreID   string
	Reader    inventory.Reader
	Inspector queue.Inspector
}

// Compatibility checks real registered handlers and execution queues using
// bounded read-only store connections. Payloads never leave the inspection.
func (c *Controller) Compatibility(ctx context.Context, workers []WorkerRecord) error {
	for _, store := range c.Participants {
		reader, ok := store.Reader.(inventory.TaskTypeReader)
		if !ok {
			return ErrEvidence
		}
		tasks, err := reader.TaskTypes(ctx)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			supported := false
			for _, worker := range workers {
				if worker.StoreID == store.StoreID && worker.ConfigurationRevision == c.Target.ConfigurationRevision && slices.Contains(worker.Queues, task.Queue) && slices.Contains(worker.Tasks, task.Name) {
					supported = true
					break
				}
			}
			if !supported {
				return errors.New("queued work has no compatible registered handler")
			}
		}
	}
	return nil
}
