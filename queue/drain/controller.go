package drain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue/inventory"
)

func ConfigurationRevision(cfg config.Configuration) string {
	identity, _ := json.Marshal(struct {
		Protocol, Release, Scope, Active, Previous, RedisStore, PostgresStore, ExecutorRole, RedisExecutor, RedisController string
	}{"queue-control-v2", convoy.GetVersion(), cfg.QueueStoreScope, string(cfg.QueueProvider), string(cfg.PreviousQueueProvider), inventory.StoreID(cfg, "redis"), inventory.StoreID(cfg, "postgres"),
		cfg.Queue.Drain.PostgresExecutorUsername, cfg.Queue.Drain.RedisExecutorUsername, cfg.Queue.Drain.RedisControllerUsername})
	sum := sha256.Sum256(identity)
	return hex.EncodeToString(sum[:16])
}

type Controller struct {
	Participants []Participant
	Repository   *Repository
	Target       Target
	Fence        Fence
	Reader       inventory.Reader
	Previous     bool
}

type Status struct {
	PausedQueues          []string            `json:"paused_queues"`
	Operation             *Operation          `json:"operation"`
	ConfigurationRevision string              `json:"configuration_revision"`
	Snapshot              *inventory.Snapshot `json:"snapshot"`
	Workers               []WorkerRecord      `json:"workers"`
	Unsettled             int64               `json:"unsettled"`
	Blockers              []string            `json:"blockers"`
	Actions               []string            `json:"actions"`
}

func (c *Controller) Status(ctx context.Context) (Status, error) {
	s := Status{ConfigurationRevision: c.Target.ConfigurationRevision, Blockers: []string{}, Actions: []string{}}
	var err error
	s.Operation, err = c.Repository.Current(ctx, c.Target.Scope)
	if err != nil {
		return s, err
	}
	s.Workers, err = c.Repository.Workers(ctx, c.Target.Scope)
	if err != nil {
		return s, err
	}
	snapshot, inspectionErr := c.Reader.Inspect(ctx)
	if inspectionErr != nil || snapshot.ObservedAt.IsZero() {
		s.Blockers = append(s.Blockers, "Queue inspection is unavailable.")
	} else {
		s.Snapshot = &snapshot
		for _, q := range snapshot.Queues {
			if q.Paused {
				s.PausedQueues = append(s.PausedQueues, q.Name)
			}
		}
	}
	if err = c.Fence.Check(ctx); err != nil {
		s.Blockers = append(s.Blockers, "Store fencing configuration could not be verified.")
	}
	count := 0
	previousCount := 0
	for _, w := range s.Workers {
		if w.StoreID == c.Target.OtherStoreID {
			previousCount++
		}
		if w.StoreID != c.Target.StoreID && w.StoreID != c.Target.OtherStoreID {
			continue
		}
		if w.StoreID == c.Target.StoreID {
			count++
		}
		if w.ConfigurationRevision != c.Target.ConfigurationRevision {
			s.Blockers = append(s.Blockers, "A worker is running a different configuration.")
		}
		if time.Since(w.ObservedAt) > 10*time.Second {
			s.Blockers = append(s.Blockers, "A worker is unreachable; its authority has not been reconciled.")
		}
	}
	if c.Target.OtherStoreID != "" && previousCount == 0 {
		s.Blockers = append(s.Blockers, "No registered worker is available for the previous store.")
	}
	if count == 0 {
		s.Blockers = append(s.Blockers, "No registered worker is available for this store.")
	}
	if err = c.Compatibility(ctx, s.Workers); err != nil {
		s.Blockers = append(s.Blockers, "Queued task compatibility could not be verified against registered workers.")
	}
	if s.Operation != nil {
		s.Unsettled, err = c.Repository.Unsettled(ctx, *s.Operation, false)
		if err != nil {
			return s, err
		}
		if s.Operation.ConfigurationRevision != c.Target.ConfigurationRevision && s.Operation.State != Resumed && (s.Operation.State != Drained || s.Operation.Purpose != DrainPrevious) {
			s.Blockers = append(s.Blockers, "The operation belongs to an earlier configuration.")
		}
		if s.Operation.StoreID == c.Target.StoreID && s.Operation.State != Resumed && (s.Operation.State != Drained || s.Operation.Purpose != DrainPrevious) {
			if s.Operation.Purpose == DrainPrevious {
				switch s.Operation.State {
				case Stopped:
					s.Actions = append(s.Actions, "resume_drain")
				case Draining, Fencing, Settling:
					s.Actions = append(s.Actions, "stop")
				}
			} else if s.Operation.State != Resuming {
				s.Actions = append(s.Actions, "resume_traffic")
			}
			return s, nil
		}
		if s.Operation.State != Resumed && (s.Operation.State != Drained || s.Operation.Purpose != DrainPrevious) {
			s.Blockers = append(s.Blockers, "Another queue operation is active.")
		}
	}
	if len(s.Blockers) == 0 {
		if c.Previous {
			s.Actions = append(s.Actions, "drain_previous")
		} else {
			s.Actions = append(s.Actions, "quiesce", "drain")
		}
	}
	if s.Operation != nil && s.Operation.StoreID != c.Target.StoreID {
		s.Operation = nil
	}
	return s, nil
}

func (c *Controller) Begin(ctx context.Context, purpose Purpose, key, revision string) (Operation, error) {
	return c.BeginWithOptions(ctx, purpose, key, revision, false)
}
func (c *Controller) BeginWithOptions(ctx context.Context, purpose Purpose, key, revision string, resumePaused bool) (Operation, error) {
	if revision != c.Target.ConfigurationRevision {
		return Operation{}, ErrStale
	}
	if c.Previous != (purpose == DrainPrevious) {
		return Operation{}, ErrConflict
	}
	if key == "" || len(key) > 200 {
		return Operation{}, ErrConflict
	}
	existing, lookupErr := c.Repository.Request(ctx, c.Target.Scope, key)
	if lookupErr == nil {
		if existing.StoreID != c.Target.StoreID || existing.ConfigurationRevision != revision || existing.Purpose != purpose || existing.ResumePaused != resumePaused {
			return Operation{}, ErrConflict
		}
		return existing, nil
	}
	if !errors.Is(lookupErr, ErrNotFound) {
		return Operation{}, lookupErr
	}
	status, err := c.Status(ctx)
	if err != nil {
		return Operation{}, err
	}
	if len(status.Blockers) > 0 {
		return Operation{}, ErrEvidence
	}
	if len(status.PausedQueues) > 0 && purpose != Quiesce && !resumePaused {
		return Operation{}, ErrEvidence
	}
	target := c.Target
	target.Purpose = purpose
	target.ResumePaused = resumePaused
	if resumePaused && purpose != Quiesce {
		target.PausedQueues, err = c.pausedManifest(ctx)
		if err != nil {
			return Operation{}, err
		}
	}
	return c.Repository.Begin(ctx, target, key)
}

func (c *Controller) Command(ctx context.Context, id string, revision int64, command string) (Operation, error) {
	op, err := c.Repository.Get(ctx, c.Target.Scope, id)
	if err != nil {
		return Operation{}, err
	}
	if op.StoreID != c.Target.StoreID || op.ConfigurationRevision != c.Target.ConfigurationRevision {
		return Operation{}, ErrStale
	}
	var next State
	switch command {
	case "stop":
		next = Stopping
	case "resume_drain":
		next = Fencing
	case "resume_traffic":
		next = Resuming
	default:
		return Operation{}, ErrConflict
	}
	return c.Repository.Advance(ctx, op.Scope, op.ID, revision, next, Evidence{})
}

func (c *Controller) Reconcile(ctx context.Context) error {
	op, err := c.Repository.Current(ctx, c.Target.Scope)
	if err != nil || op == nil {
		return err
	}
	if op.StoreID != c.Target.StoreID {
		return nil
	}
	if op.State == Resumed || op.State == Quiesced || op.State == Drained || op.State == Stopped {
		return nil
	}
	if op.ConfigurationRevision != c.Target.ConfigurationRevision {
		return ErrStale
	}
	if op.State == Resuming {
		workers, readErr := c.Repository.Workers(ctx, op.Scope)
		if readErr != nil {
			return readErr
		}
		ready := 0
		for _, w := range workers {
			if !op.IncludesStore(w.StoreID, op.Provider) {
				continue
			}
			if w.ConfigurationRevision != op.ConfigurationRevision || w.OperationID != op.ID || w.Revision != op.Revision || w.State != expectedWorkerState(*op, w.StoreID) {
				return ErrEvidence
			}
			ready++
		}
		if ready == 0 {
			return ErrEvidence
		}

		if err := c.restorePause(ctx, *op, true); err != nil {
			return err
		}
		if err := c.Fence.Open(ctx, *op); err != nil {
			return err
		}
		closed, checkErr := c.Fence.Closed(ctx, *op)
		if checkErr != nil {
			return checkErr
		}
		if closed {
			return ErrEvidence
		}
		_, err = c.Repository.Advance(ctx, op.Scope, op.ID, op.Revision, Resumed, Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, RuntimeReady: true, AdmissionOpen: true})
		return err
	}
	if op.State == Fencing {
		n, countErr := c.Repository.Unsettled(ctx, *op, true)
		if countErr != nil {
			return countErr
		}
		if n != 0 {
			return nil
		}
		if err := c.Fence.Close(ctx, *op); err != nil {
			return err
		}
	}
	e := Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, AdmissionClosed: true}
	e.WritersFenced, err = c.Fence.Closed(ctx, *op)
	if err != nil {
		return err
	}
	snapshot, err := c.Reader.Inspect(ctx)
	if err != nil {
		return err
	}
	e.InspectionSucceeded = !snapshot.ObservedAt.IsZero()
	for _, q := range snapshot.Queues {
		e.Pending += q.Pending
		e.Processing += q.Processing
		e.Future += q.Scheduled + q.Retry + q.Future
		e.Aggregating += q.Aggregating
		e.Archived += q.Archived
		e.Unknown += q.Unknown
	}
	workers, err := c.Repository.Workers(ctx, op.Scope)
	if err != nil {
		return err
	}
	e.ClaimsStopped = true
	count := 0
	for _, w := range workers {
		if !op.IncludesStore(w.StoreID, op.Provider) {
			continue
		}
		count++
		if w.ConfigurationRevision != op.ConfigurationRevision {
			return ErrStale
		}
		if w.OperationID != op.ID || w.Revision != op.Revision || w.State != "stopped" {
			e.ClaimsStopped = false
		}
	}
	if count == 0 {
		return ErrEvidence
	}
	var next State
	switch op.State {
	case Fencing:
		if err := c.Compatibility(ctx, workers); err != nil {
			return err
		}
		if op.Purpose == DrainPrevious && e.Processing != 0 {
			return nil
		}
		if op.Purpose != Quiesce {
			if err := c.restorePause(ctx, *op, false); err != nil {
				return err
			}
		}
		next = Draining
		if op.Purpose == Quiesce {
			next = Quiescing
		}
	case Quiescing, Settling:
		next = Verifying
	case Draining:
		next = Settling
	case Verifying:
		if err := c.restorePause(ctx, *op, true); err != nil {
			return err
		}
		next = Drained
		if op.Purpose == Quiesce {
			next = Quiesced
		}
	case Stopping:
		if e.ClaimsStopped {
			if err := c.restorePause(ctx, *op, true); err != nil {
				return err
			}
		}
		next = Stopped
	default:
		return ErrConflict
	}
	_, err = c.Repository.Advance(ctx, op.Scope, op.ID, op.Revision, next, e)
	if errors.Is(err, ErrEvidence) {
		return nil
	}
	return err
}

func expectedWorkerState(op Operation, storeID string) string {
	if storeID == op.OtherStoreID {
		return "stopped"
	}
	return "running"
}
