package drain

import (
	"context"
	"encoding/json"
)

type pausedQueue struct {
	StoreID string `json:"store_id"`
	Name    string `json:"name"`
}

func (c *Controller) pausedManifest(ctx context.Context) (string, error) {
	var queues []pausedQueue
	for _, store := range c.Participants {
		snapshot, err := store.Reader.Inspect(ctx)
		if err != nil {
			return "", err
		}
		for _, q := range snapshot.Queues {
			if q.Paused {
				queues = append(queues, pausedQueue{StoreID: store.StoreID, Name: q.Name})
			}
		}
	}
	data, err := json.Marshal(queues)
	return string(data), err
}
func (c *Controller) restorePause(ctx context.Context, op Operation, paused bool) error {
	if op.PausedQueues == "" {
		return nil
	}
	var queues []pausedQueue
	if err := json.Unmarshal([]byte(op.PausedQueues), &queues); err != nil {
		return err
	}
	for _, q := range queues {
		matched := false
		for _, store := range c.Participants {
			if store.StoreID != q.StoreID {
				continue
			}
			matched = true
			var err error
			if paused {
				err = store.Inspector.PauseQueue(ctx, q.Name)
			} else {
				err = store.Inspector.UnpauseQueue(ctx, q.Name)
			}
			if err != nil {
				return err
			}
		}
		if !matched {
			return ErrStale
		}
	}
	return nil
}
