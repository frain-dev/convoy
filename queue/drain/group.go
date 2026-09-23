package drain

import (
	"context"

	"github.com/frain-dev/convoy/queue/inventory"
)

// StoreGroup includes the previous store in a deployment-wide barrier. Resume
// opens only active admission; a retired store remains fenced.
type StoreGroup struct{ Active, Previous Fence }

func (g StoreGroup) Bind(ctx context.Context) error {
	if err := g.Active.Bind(ctx); err != nil {
		return err
	}
	return g.Previous.Bind(ctx)
}
func (g StoreGroup) Check(ctx context.Context) error {
	if err := g.Active.Check(ctx); err != nil {
		return err
	}
	return g.Previous.Check(ctx)
}
func (g StoreGroup) Close(ctx context.Context, op Operation) error {
	if err := g.Active.Close(ctx, op); err != nil {
		return err
	}
	return g.Previous.Close(ctx, op)
}
func (g StoreGroup) Open(ctx context.Context, op Operation) error { return g.Active.Open(ctx, op) }
func (g StoreGroup) Closed(ctx context.Context, op Operation) (bool, error) {
	closed, err := g.Active.Closed(ctx, op)
	if err != nil || !closed {
		return closed, err
	}
	return g.Previous.Closed(ctx, op)
}

type StoreReaders struct{ Active, Previous inventory.Reader }

func (g StoreReaders) Inspect(ctx context.Context) (inventory.Snapshot, error) {
	active, err := g.Active.Inspect(ctx)
	if err != nil {
		return active, err
	}
	previous, err := g.Previous.Inspect(ctx)
	if err != nil {
		return inventory.Snapshot{}, err
	}
	for i := range previous.Queues {
		previous.Queues[i].Name = "Previous / " + previous.Queues[i].Name
	}
	active.Queues = append(active.Queues, previous.Queues...)
	if previous.ObservedAt.Before(active.ObservedAt) {
		active.ObservedAt = previous.ObservedAt
	}
	return active, nil
}
