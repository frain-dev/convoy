package drain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

// Gate accounts for accepted producers and handler descendants across process
// boundaries. It complements store-level fencing; it cannot revoke a client
// which bypasses this protocol.
type Gate struct {
	Repository *Repository
	Scope      string
	StoreID    string
	Previous   bool
	workerID   atomic.Value
}

type admissionKey struct{}

type admittedWork struct {
	gate             *Gate
	mu               sync.Mutex
	accepting        bool
	uncertain        bool
	preserveExisting bool
	children         sync.WaitGroup
}

// Run admits work before invoking it. Ambiguous producer writes retain their
// receipt. A normal handler error retains its exact type for the queue retry
// policy; the broker still accounts for that claimed job and its next state.
func (g *Gate) Run(ctx context.Context, kind Admission, work func(context.Context) error) error {
	if parent, ok := ctx.Value(admissionKey{}).(*admittedWork); ok {
		if parent.gate.Scope != g.Scope || parent.gate.StoreID != g.StoreID {
			return errors.New("accepted queue work cannot change stores")
		}
		parent.mu.Lock()
		if !parent.accepting {
			parent.mu.Unlock()
			return ErrFenced
		}
		parent.children.Add(1)
		parent.mu.Unlock()
		defer parent.children.Done()
		err := work(ctx)
		if err != nil {
			parent.mu.Lock()
			parent.uncertain = true
			parent.mu.Unlock()
		}
		return err
	}
	if g.Previous && kind == Producer {
		return ErrFenced
	}
	receipt, err := g.Repository.admit(ctx, g.Scope, g.StoreID, kind, g.worker())
	if err != nil {
		return fmt.Errorf("%w: admission unavailable: %v", ErrFenced, err)
	}
	accepted := &admittedWork{gate: g, accepting: true, preserveExisting: kind == Claim}
	workErr := work(context.WithValue(ctx, admissionKey{}, accepted))
	accepted.mu.Lock()
	accepted.accepting = false
	accepted.mu.Unlock()
	accepted.children.Wait()
	if accepted.uncertain && workErr == nil {
		return ErrEvidence
	}
	if accepted.uncertain || (kind == Producer && workErr != nil) {
		return workErr
	}
	// Client cancellation cannot discard an already committed result. Bound
	// reconciliation separately while leaving its receipt outstanding on error.
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	settleErr := g.Repository.Settle(settleCtx, receipt)
	if workErr != nil {
		// Preserve the exact handler error so queue rate/circuit deferrals
		// retain their delay and retry budget. A failed receipt settlement
		// remains a durable completion blocker independent of that retry.
		return workErr
	}
	return settleErr
}

// GuardQueue routes enqueue and archive-cleanup operations through admission.
// Handler descendants reuse their parent's receipt. Inspection remains on the
// broker's separate read-only inspector dependency.
func (g *Gate) GuardQueue(q queue.Queuer) queue.Queuer {
	return &guardedQueue{Queuer: q, gate: g}
}

type guardedQueue struct {
	queue.Queuer
	gate *Gate
}

// Queue replacement removes old jobs before writing replacements. The removal
// must share its caller's admission receipt so a fence or crash between the
// two steps cannot be mistaken for a completed drain.
func (q *guardedQueue) DeleteEventDeliveriesFromQueueContext(ctx context.Context, name convoy.QueueName, ids []string) error {
	remover, ok := q.Queuer.(interface {
		DeleteEventDeliveriesFromQueue(convoy.QueueName, []string) error
	})
	if !ok {
		return nil
	}
	return q.gate.Run(ctx, Producer, func(context.Context) error {
		return remover.DeleteEventDeliveriesFromQueue(name, ids)
	})
}

func (q *guardedQueue) Write(ctx context.Context, task convoy.TaskName, name convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	return q.gate.Run(ctx, Producer, func(accepted context.Context) error {
		jobCopy := *job
		if admission, ok := accepted.Value(admissionKey{}).(*admittedWork); ok {
			jobCopy.PreserveExisting = jobCopy.PreserveExisting || admission.preserveExisting
		}
		return q.Queuer.Write(accepted, task, name, &jobCopy)
	})
}

func (q *guardedQueue) WriteWithoutTimeout(ctx context.Context, task convoy.TaskName, name convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	return q.gate.Run(ctx, Producer, func(accepted context.Context) error {
		jobCopy := *job
		if admission, ok := accepted.Value(admissionKey{}).(*admittedWork); ok {
			jobCopy.PreserveExisting = jobCopy.PreserveExisting || admission.preserveExisting
		}
		return q.Queuer.WriteWithoutTimeout(accepted, task, name, &jobCopy)
	})
}

func (q *guardedQueue) DeleteArchived(ctx context.Context) error {
	op, err := q.gate.Repository.Current(ctx, q.gate.Scope)
	if err != nil {
		return err
	}
	if op != nil && op.State != Resumed && (op.Purpose != DrainPrevious || op.StoreID == q.gate.StoreID) {
		return ErrFenced
	}
	archiver, ok := q.Queuer.(queue.Archiver)
	if !ok {
		return errors.New("queue archive cleanup is unavailable")
	}
	return q.gate.Run(ctx, Producer, archiver.DeleteArchived)
}

func (g *Gate) worker() string      { v, _ := g.workerID.Load().(string); return v }
func (g *Gate) setWorker(id string) { g.workerID.Store(id) }

// RunBackgroundClaim permits executor housekeeping only after registration.
// It shares the claim barrier so sampling and its descendants settle before
// quiescence is acknowledged, while retries can keep progressing during drain.
func (g *Gate) RunBackgroundClaim(ctx context.Context, work func(context.Context) error) error {
	if g.worker() == "" {
		return ErrFenced
	}
	if g.Previous {
		op, err := g.Repository.Current(ctx, g.Scope)
		if err != nil {
			return err
		}
		if op == nil || op.State != Draining || (op.Purpose == DrainPrevious && op.StoreID != g.StoreID) {
			return ErrFenced
		}
	}
	return g.Run(ctx, Claim, work)
}
