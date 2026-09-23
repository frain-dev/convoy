package drain

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

// PostgresFence separates the normal application role from the dedicated
// executor role. It fences the queue tables, not the rest of the application's
// database, so a different active queue provider can keep serving traffic.
type PostgresFence struct {
	DB           *sqlx.DB
	Scope        string
	StoreID      string
	ExecutorRole string
}

// Bind belongs to explicit writer provisioning, never inventory or preflight.
// It refuses to reassign a namespace already owned by another deployment.
func (f PostgresFence) Bind(ctx context.Context) error {
	if f.Scope == "" || f.StoreID == "" || f.ExecutorRole == "" {
		return errors.New("queue fence identity and executor role are required")
	}
	tx, err := f.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role string
	if err := tx.GetContext(ctx, &role, `SELECT current_user`); err != nil {
		return err
	}
	if role == f.ExecutorRole {
		return errors.New("queue executor and normal writer roles must differ")
	}
	var permitted bool
	err = tx.GetContext(ctx, &permitted, `SELECT
		has_table_privilege($1, 'convoy.queue_jobs', 'SELECT') AND
		has_table_privilege($1, 'convoy.queue_jobs', 'INSERT') AND
		has_table_privilege($1, 'convoy.queue_jobs', 'UPDATE') AND
		has_table_privilege($1, 'convoy.queue_jobs', 'DELETE') AND
		has_table_privilege($1, 'convoy.queue_state', 'SELECT') AND
		has_table_privilege($1, 'convoy.queue_state', 'INSERT') AND
		has_table_privilege($1, 'convoy.queue_state', 'UPDATE') AND
		has_table_privilege($1, 'convoy.queue_state', 'DELETE') AND
		has_table_privilege($1, 'convoy.queue_store_fence', 'SELECT') AND
		NOT has_table_privilege($1, 'convoy.queue_store_fence', 'UPDATE') AND
		NOT has_table_privilege($1, 'convoy.queue_store_fence', 'DELETE') AND
		NOT has_table_privilege($1, 'convoy.queue_store_fence', 'INSERT') AND
		NOT has_table_privilege($1, 'convoy.queue_store_fence', 'TRUNCATE')`, f.ExecutorRole)
	if err != nil {
		return err
	}
	if !permitted {
		return errors.New("queue executor requires queue access without fence-management privileges")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO convoy.queue_store_fence(singleton,scope,store_id,executor_role)
		VALUES(TRUE,$1,$2,$3) ON CONFLICT(singleton) DO NOTHING`, f.Scope, f.StoreID, f.ExecutorRole)
	if err != nil {
		return err
	}
	var matches bool
	err = tx.GetContext(ctx, &matches, `SELECT scope=$1 AND store_id=$2 AND executor_role=$3
		FROM convoy.queue_store_fence WHERE singleton FOR UPDATE`, f.Scope, f.StoreID, f.ExecutorRole)
	if err != nil {
		return err
	}
	if !matches {
		return ErrConflict
	}
	return tx.Commit()
}

func (f PostgresFence) Close(ctx context.Context, op Operation) error {
	return f.setClosed(ctx, op, true)
}

func (f PostgresFence) Closed(ctx context.Context, op Operation) (bool, error) {
	if err := f.Check(ctx); err != nil {
		return false, err
	}
	var closed bool
	err := f.DB.GetContext(ctx, &closed, `SELECT closed AND operation_id=$4 AND epoch=$5
		FROM convoy.queue_store_fence WHERE singleton AND scope=$1 AND store_id=$2 AND executor_role=$3`, f.Scope, f.StoreID, f.ExecutorRole, op.ID, op.Epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return closed, err
}

// Open is only for a deployment-wide resume. A stopped or completed previous
// store must retain its fence and cannot be reopened by this operation.
func (f PostgresFence) Open(ctx context.Context, op Operation) error {
	return f.setClosed(ctx, op, false)
}

func (f PostgresFence) setClosed(ctx context.Context, op Operation, closed bool) error {
	if op.Scope != f.Scope || !op.IncludesStore(f.StoreID, "postgres") {
		return ErrConflict
	}
	return NewRepository(f.DB).transaction(ctx, f.Scope, func(tx *sqlx.Tx, _ int64, current string) error {
		if current != op.ID {
			return ErrStale
		}
		persisted, err := readOperation(ctx, tx, f.Scope, op.ID)
		if err != nil {
			return err
		}
		if persisted.Revision != op.Revision || persisted.Epoch != op.Epoch || persisted.Target != op.Target {
			return ErrStale
		}
		if closed && persisted.State != Fencing {
			return ErrConflict
		}
		if !closed && (persisted.State != Resuming || persisted.Purpose == DrainPrevious) {
			return ErrConflict
		}
		var result sql.Result
		if closed {
			result, err = tx.ExecContext(ctx, `UPDATE convoy.queue_store_fence
    SET closed=TRUE,operation_id=$4,epoch=$5
    WHERE singleton AND scope=$1 AND store_id=$2 AND executor_role=$3
    AND epoch <= $5`, f.Scope, f.StoreID, f.ExecutorRole, op.ID, op.Epoch)
		} else {
			result, err = tx.ExecContext(ctx, `UPDATE convoy.queue_store_fence SET closed=FALSE
    WHERE singleton AND scope=$1 AND store_id=$2 AND executor_role=$3 AND operation_id=$4 AND epoch=$5`, f.Scope, f.StoreID, f.ExecutorRole, op.ID, op.Epoch)
		}
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrStale
		}
		return nil
	})
}

// Check observes the binding without creating or changing it.
func (f PostgresFence) Check(ctx context.Context) error {
	var matches bool
	err := f.DB.GetContext(ctx, &matches, `SELECT scope=$1 AND store_id=$2 AND executor_role=$3 FROM convoy.queue_store_fence WHERE singleton`, f.Scope, f.StoreID, f.ExecutorRole)
	if err != nil {
		return err
	}
	if !matches {
		return ErrConflict
	}
	var triggers int
	err = f.DB.GetContext(ctx, &triggers, `SELECT count(*) FROM pg_trigger WHERE tgrelid IN ('convoy.queue_jobs'::regclass,'convoy.queue_state'::regclass) AND tgfoid='convoy.enforce_queue_store_fence()'::regprocedure AND tgenabled='A' AND tgtype=62 AND NOT tgisinternal`)
	if err != nil {
		return err
	}
	if triggers != 2 {
		return errors.New("queue store fence triggers are unavailable")
	}
	return nil
}
