package drain

import (
	"context"
	"database/sql/driver"
	"errors"
	"hash/fnv"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
)

// ConsumerControl is implemented by the real worker pool, including its flush
// barrier. Suspend must return only once handlers and acknowledgements settle.
type ConsumerControl interface {
	Start() error
	Suspend() error
}

type WorkerRecord struct {
	Queues                pq.StringArray `json:"queues" db:"queues"`
	Tasks                 pq.StringArray `json:"tasks" db:"tasks"`
	ID                    string         `json:"id" db:"id"`
	StoreID               string         `json:"store_id" db:"store_id"`
	ConfigurationRevision string         `json:"configuration_revision" db:"configuration_revision"`
	OperationID           string         `json:"operation_id" db:"operation_id"`
	Revision              int64          `json:"revision" db:"revision"`
	State                 string         `json:"state" db:"state"`
	ObservedAt            time.Time      `json:"observed_at" db:"observed_at"`
}

// Runtime uses durable acknowledgements, never heartbeat expiry, to prove a
// consumer has stopped. An unreachable or crashed replica remains visible.
type Runtime struct {
	mu                                    sync.Mutex
	Gate                                  *Gate
	session                               *sqlx.Conn
	Repository                            *Repository
	Scope, StoreID, ConfigurationRevision string
	Previous                              bool
	Consumer                              ConsumerControl
	id                                    string
}

func (r *Runtime) Register(ctx context.Context) error {
	if r.id != "" {
		return nil
	}
	id := ulid.Make().String()
	session, err := r.Repository.db.Connx(ctx)
	if err != nil {
		return err
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(r.Scope + "/" + id))
	key := int64(hash.Sum64() & 0x7fffffffffffffff)
	var pid int
	if err = session.GetContext(ctx, &pid, `SELECT pg_backend_pid()`); err != nil {
		_ = session.Close()
		return err
	}
	if _, err = session.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		_ = session.Close()
		return err
	}
	queues, tasks := []string{}, []string{}
	if capable, ok := r.Consumer.(interface{ Capabilities() ([]string, []string) }); ok {
		queues, tasks = capable.Capabilities()
	}
	err = r.Repository.transaction(ctx, r.Scope, func(tx *sqlx.Tx, _ int64, _ string) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO convoy.queue_operation_workers(scope,id,store_id,configuration_revision,state,backend_pid,lease_key,queues,tasks) VALUES($1,$2,$3,$4,'starting',$5,$6,$7,$8)`, r.Scope, id, r.StoreID, r.ConfigurationRevision, pid, key, pq.Array(queues), pq.Array(tasks))
		return err
	})
	if err == nil {
		r.id = id
		r.session = session
		if r.Gate != nil {
			r.Gate.setWorker(id)
		}
	} else {
		_, _ = session.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, key)
		_ = session.Close()
	}

	return err
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Register(ctx); err != nil {
		return err
	}
	var alive int
	if err := r.session.GetContext(ctx, &alive, `SELECT 1`); err != nil {
		_ = r.Consumer.Suspend()
		_ = r.session.Conn.Raw(func(interface{}) error { return driver.ErrBadConn })
		_ = r.session.Close()
		r.session = nil
		r.id = ""
		if r.Gate != nil {
			r.Gate.setWorker("")
		}
		return err
	}
	op, err := r.Repository.Current(ctx, r.Scope)
	if err != nil {
		_ = r.Consumer.Suspend()
		return err
	}
	run := !r.Previous
	if op != nil && (op.Purpose != DrainPrevious || op.StoreID == r.StoreID) {
		if op.ConfigurationRevision != r.ConfigurationRevision && op.State != Resumed && (op.State != Drained || op.Purpose != DrainPrevious) {
			_ = r.Consumer.Suspend()
			return ErrStale
		}
		run = op.State == Draining || (!r.Previous && (op.State == Resumed || op.State == Resuming || (op.State == Fencing && op.Purpose != Quiesce)))
	}
	state := "stopped"
	if run {
		if err := r.Consumer.Start(); err != nil {
			return err
		}
		state = "running"
	} else {
		if err := r.Consumer.Suspend(); err != nil {
			return err
		}
	}
	return r.acknowledge(ctx, op, state)
}

func (r *Runtime) acknowledge(ctx context.Context, op *Operation, state string) error {
	return r.Repository.transaction(ctx, r.Scope, func(tx *sqlx.Tx, _ int64, current string) error {
		id := ""
		revision := int64(0)
		if op != nil {
			id = op.ID
			revision = op.Revision
		}
		if current != id {
			return ErrStale
		}
		if op != nil {
			latest, err := readOperation(ctx, tx, r.Scope, id)
			if err != nil {
				return err
			}
			if latest.Revision != revision {
				return ErrStale
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE convoy.queue_operation_workers SET operation_id=$3,revision=$4,state=$5,observed_at=clock_timestamp() WHERE scope=$1 AND id=$2`, r.Scope, r.id, id, revision, state)
		return err
	})
}

func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Consumer.Suspend(); err != nil {
		return err
	}
	if r.id == "" {
		return nil
	}
	_, err := r.Repository.db.ExecContext(ctx, `UPDATE convoy.queue_operation_workers SET state='closed',observed_at=clock_timestamp() WHERE scope=$1 AND id=$2`, r.Scope, r.id)
	if r.session != nil {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if _, err := r.session.ExecContext(release, `SELECT pg_advisory_unlock_all()`); err != nil {
			_ = r.session.Conn.Raw(func(interface{}) error { return driver.ErrBadConn })
		}
		cancel()
		_ = r.session.Close()
		r.session = nil
	}
	return err
}

func (r *Repository) Workers(ctx context.Context, scope string) ([]WorkerRecord, error) {
	result := []WorkerRecord{}
	err := r.transaction(ctx, scope, func(tx *sqlx.Tx, _ int64, _ string) error {
		_, err := tx.ExecContext(ctx, `UPDATE convoy.queue_operation_workers w SET state='closed',observed_at=clock_timestamp() WHERE scope=$1 AND state<>'closed' AND NOT (`+workerSessionExists+`)`, scope)
		if err != nil {
			return err
		}
		return tx.SelectContext(ctx, &result, `SELECT id,store_id,configuration_revision,operation_id,revision,state,observed_at,queues,tasks FROM convoy.queue_operation_workers WHERE scope=$1 AND state<>'closed' ORDER BY id`, scope)
	})
	return result, err
}

func (r *Repository) Unsettled(ctx context.Context, op Operation, producersOnly bool) (int64, error) {
	var n int64
	err := r.db.GetContext(ctx, &n, `SELECT (SELECT count(*) FROM convoy.queue_operation_receipts
 WHERE scope=$1 AND settled_at IS NULL AND (NOT $2 OR kind='producer') AND (NOT $3 OR store_id=$4)) +
 (SELECT count(*) FROM convoy.queue_execution_ownership WHERE scope=$1 AND state='running' AND NOT $2 AND (NOT $3 OR store_id=$4))`, op.Scope, producersOnly, op.Purpose == DrainPrevious, op.StoreID)
	return n, err
}

// Run stays fail-closed on store outages. Reconciliation retries are bounded by
// the caller's context; an outage cannot invent a stopped acknowledgement.
func (r *Runtime) Run(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := r.Reconcile(check)
			cancel()
			if err != nil && !errors.Is(err, ErrStale) && onError != nil {
				onError(err)
			}
		}
	}
}

const workerSessionExists = `EXISTS (SELECT 1 FROM pg_locks l WHERE l.locktype='advisory' AND l.granted AND l.pid=w.backend_pid
 AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND l.classid=(w.lease_key >> 32)::oid AND l.objid=(w.lease_key & 4294967295)::oid AND l.objsubid=1)`
