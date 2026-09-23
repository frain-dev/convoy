package drain

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

// Repository serializes admission and commands on a deployment-scoped row.
// Durable receipts span the application database/broker boundary: an ambiguous
// enqueue leaves a receipt outstanding rather than permitting false completion.
// A broker adapter still has to enforce writer authority at the store itself.
type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) transaction(ctx context.Context, scope string, f func(*sqlx.Tx, int64, string) error) error {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO convoy.queue_operation_scopes(scope) VALUES ($1) ON CONFLICT DO NOTHING`, scope)
	if err != nil {
		return err
	}
	var epoch int64
	var current sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT epoch, current_operation FROM convoy.queue_operation_scopes WHERE scope=$1 FOR UPDATE`, scope).Scan(&epoch, &current)
	if err != nil {
		return err
	}
	if err := f(tx, epoch, current.String); err != nil {
		return err
	}
	return tx.Commit()
}

type documentReader interface {
	GetContext(context.Context, interface{}, string, ...interface{}) error
}

func readOperation(ctx context.Context, db documentReader, scope, id string) (Operation, error) {
	var raw []byte
	err := db.GetContext(ctx, &raw, `SELECT document FROM convoy.queue_operations WHERE scope=$1 AND id=$2`, scope, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	if err != nil {
		return Operation{}, err
	}
	var op Operation
	err = json.Unmarshal(raw, &op)
	return op, err
}

func (r *Repository) Get(ctx context.Context, scope, id string) (Operation, error) {
	return readOperation(ctx, r.db, scope, id)
}

func (r *Repository) Current(ctx context.Context, scope string) (*Operation, error) {
	var id sql.NullString
	err := r.db.GetContext(ctx, &id, `SELECT current_operation FROM convoy.queue_operation_scopes WHERE scope=$1`, scope)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !id.Valid {
		return nil, nil
	}
	op, err := r.Get(ctx, scope, id.String)
	return &op, err
}

// Begin is idempotent even after an operation was resumed. Reusing a key with
// another target is an error; it never retargets an existing operation.
func (r *Repository) Begin(ctx context.Context, target Target, key string) (Operation, error) {
	if err := target.validate(); err != nil {
		return Operation{}, err
	}
	if key == "" || len(key) > 200 {
		return Operation{}, errors.New("an idempotency key of at most 200 characters is required")
	}
	var result Operation
	err := r.transaction(ctx, target.Scope, func(tx *sqlx.Tx, epoch int64, current string) error {
		var previous string
		err := tx.GetContext(ctx, &previous, `SELECT id FROM convoy.queue_operations WHERE scope=$1 AND idempotency_key=$2`, target.Scope, key)
		if err == nil {
			result, err = readOperation(ctx, tx, target.Scope, previous)
			if err != nil {
				return err
			}
			if result.Target != target {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if current != "" {
			op, readErr := readOperation(ctx, tx, target.Scope, current)
			if readErr != nil {
				return readErr
			}
			if op.State != Resumed && (op.State != Drained || op.Purpose != DrainPrevious) {
				return ErrConflict
			}
		}
		var now time.Time
		if err := tx.GetContext(ctx, &now, `SELECT clock_timestamp()`); err != nil {
			return err
		}
		result = Operation{ID: ulid.Make().String(), Target: target, Epoch: epoch + 1, Revision: 1, State: Fencing, CreatedAt: now, UpdatedAt: now}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO convoy.queue_operations(id,scope,idempotency_key,document) VALUES($1,$2,$3,$4)`, result.ID, target.Scope, key, raw)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE convoy.queue_operation_scopes SET epoch=$2,current_operation=$3 WHERE scope=$1`, target.Scope, result.Epoch, result.ID)
		if err != nil {
			return err
		}
		return appendHistory(ctx, tx, result)
	})
	return result, err
}

func appendHistory(ctx context.Context, tx *sqlx.Tx, op Operation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO convoy.queue_operation_history(operation_id,revision,state,actor) VALUES($1,$2,$3,$4)`, op.ID, op.Revision, op.State, actor(ctx))
	return err
}

// Advance is for the runtime coordinator. Administrative handlers must not
// accept Evidence from a request body. The expected revision prevents an old
// controller's response from undoing a concurrent Stop or Resume command.
func (r *Repository) Advance(ctx context.Context, scope, id string, revision int64, next State, evidence Evidence) (Operation, error) {
	var result Operation
	err := r.transaction(ctx, scope, func(tx *sqlx.Tx, _ int64, current string) error {
		if current != id {
			return ErrStale
		}
		var err error
		result, err = readOperation(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if next == result.State && (result.Revision == revision || result.Revision == revision+1) {
			return nil
		}
		if result.Revision != revision {
			return ErrStale
		}
		// The store supplies the unsettled count itself, under the same lock
		// used by admission. A controller cannot accidentally omit receipts.
		var unsettled int64
		// Consumers may keep finishing accepted work during a full drain's
		// admission barrier. Only external producers must settle to leave
		// Fencing; verification and Stop must account for every receipt.
		producersOnly := next == Quiescing || next == Draining
		err = tx.GetContext(ctx, &unsettled, `SELECT count(*) FROM convoy.queue_operation_receipts
			WHERE scope=$1 AND settled_at IS NULL AND (NOT $2 OR kind='producer')
			AND (NOT $3 OR store_id=$4)`, scope, producersOnly, result.Purpose == DrainPrevious, result.StoreID)
		if err != nil {
			return err
		}
		if !producersOnly {
			var running int64
			if err := tx.GetContext(ctx, &running, `SELECT count(*) FROM convoy.queue_execution_ownership WHERE scope=$1 AND state='running' AND (NOT $2 OR store_id=$3)`, scope, result.Purpose == DrainPrevious, result.StoreID); err != nil {
				return err
			}
			unsettled += running
		}
		if unsettled != 0 {
			// Preserve any coordinator-reported blockers as well. Replacing its
			// count with the receipt count would hide buffers outside this store.
			evidence.Unsettled = unsettled
		}
		if err := validateTransition(result, next, evidence); err != nil {
			return err
		}
		result.State = next
		result.Revision++
		if err := tx.GetContext(ctx, &result.UpdatedAt, `SELECT clock_timestamp()`); err != nil {
			return err
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE convoy.queue_operations SET document=$3 WHERE scope=$1 AND id=$2`, scope, id, raw)
		if err != nil {
			return err
		}
		return appendHistory(ctx, tx, result)
	})
	return result, err
}

type Receipt struct {
	ID      string
	Scope   string
	StoreID string
	Epoch   int64
}

type Admission string

const (
	Producer Admission = "producer"
	Claim    Admission = "claim"
)

// Admit must precede external side effects. Worker descendants stay within
// their parent's unsettled receipt, which is settled only after all writes and
// acknowledgements finish. Receipts do not expire on process death.
func (r *Repository) Admit(ctx context.Context, scope, storeID string, kind Admission) (Receipt, error) {
	return r.admit(ctx, scope, storeID, kind, "")
}
func (r *Repository) admit(ctx context.Context, scope, storeID string, kind Admission, workerID string) (Receipt, error) {
	if scope == "" || len(scope) > 200 || storeID == "" || len(storeID) > 200 || (kind != Producer && kind != Claim) {
		return Receipt{}, errors.New("invalid queue admission scope or kind")
	}
	var result Receipt
	err := r.transaction(ctx, scope, func(tx *sqlx.Tx, epoch int64, current string) error {
		if kind == Claim && workerID != "" {
			var authorized bool
			err := tx.GetContext(ctx, &authorized, `SELECT EXISTS(SELECT 1 FROM convoy.queue_operation_workers w WHERE scope=$1 AND id=$2 AND store_id=$3 AND state<>'closed' AND (`+workerSessionExists+`))`, scope, workerID, storeID)
			if err != nil {
				return err
			}
			if !authorized {
				return ErrFenced
			}
		}
		if current != "" {
			op, err := readOperation(ctx, tx, scope, current)
			if err != nil {
				return err
			}
			selected := op.Purpose != DrainPrevious || op.StoreID == storeID
			if selected && op.State != Resumed {
				claimsAllowed := op.State == Draining || (op.State == Fencing && op.Purpose != DrainPrevious)
				if kind == Producer || !claimsAllowed {
					return ErrFenced
				}
			}
		}
		result = Receipt{ID: ulid.Make().String(), Scope: scope, StoreID: storeID, Epoch: epoch}
		_, err := tx.ExecContext(ctx, `INSERT INTO convoy.queue_operation_receipts(id,scope,store_id,epoch,kind) VALUES($1,$2,$3,$4,$5)`, result.ID, scope, storeID, epoch, kind)
		return err
	})
	return result, err
}

// Settle removes a receipt only after its work has a known durable outcome.
// Unsettled receipts survive crashes and remain completion blockers. A retry
// after deletion is safe, including after restart.
func (r *Repository) Settle(ctx context.Context, receipt Receipt) error {
	return r.transaction(ctx, receipt.Scope, func(tx *sqlx.Tx, _ int64, _ string) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM convoy.queue_operation_receipts WHERE id=$1 AND scope=$2 AND epoch=$3 AND store_id=$4`, receipt.ID, receipt.Scope, receipt.Epoch, receipt.StoreID)
		return err
	})
}

type Transition struct {
	Actor      string    `json:"actor" db:"actor"`
	Revision   int64     `json:"revision" db:"revision"`
	State      State     `json:"state" db:"state"`
	ObservedAt time.Time `json:"observed_at" db:"observed_at"`
}

func (r *Repository) History(ctx context.Context, scope, id string) ([]Transition, error) {
	if _, err := r.Get(ctx, scope, id); err != nil {
		return nil, err
	}
	result := []Transition{}
	err := r.db.SelectContext(ctx, &result, `SELECT revision,state,actor,observed_at FROM convoy.queue_operation_history WHERE operation_id=$1 ORDER BY revision`, id)
	return result, err
}

func (r *Repository) Request(ctx context.Context, scope, key string) (Operation, error) {
	var id string
	err := r.db.GetContext(ctx, &id, `SELECT id FROM convoy.queue_operations WHERE scope=$1 AND idempotency_key=$2`, scope, key)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	if err != nil {
		return Operation{}, err
	}
	return r.Get(ctx, scope, id)
}
