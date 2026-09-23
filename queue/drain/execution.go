package drain

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

// Execute serializes the same task across stores on the application database.
// A process loss leaves running ownership unresolved; another store must never
// turn a lost connection into permission to repeat its external effects.
func (r *Repository) Execute(ctx context.Context, scope, storeID, taskName, taskID string, run func(context.Context) error) error {
	if taskID == "" || taskName == "" {
		return fmt.Errorf("%w: task identity is unavailable", ErrFenced)
	}
	connection, err := r.db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("%w: execution authority is unavailable", ErrFenced)
	}
	defer connection.Close()
	lockName := taskName
	delivery := taskName == string(convoy.EventProcessor) || taskName == string(convoy.RetryEventProcessor)
	if delivery {
		lockName = "delivery"
	}
	digest := sha256.Sum256([]byte(scope + "/" + lockName + "/" + taskID))
	key := int64(binary.BigEndian.Uint64(digest[:8]))
	var locked bool
	if err = connection.GetContext(ctx, &locked, `SELECT pg_try_advisory_lock($1)`, key); err != nil || !locked {
		return fmt.Errorf("%w: task is owned by another executor", ErrFenced)
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, unlockErr := connection.ExecContext(release, `SELECT pg_advisory_unlock($1)`, key); unlockErr != nil {
			// A connection with an uncertain session lock must not return to the pool.
			_ = connection.Conn.Raw(func(interface{}) error { return driver.ErrBadConn })
		}
	}()
	var owner struct {
		StoreID string `db:"store_id"`
		State   string `db:"state"`
	}
	err = connection.GetContext(ctx, &owner, `SELECT store_id,state FROM convoy.queue_execution_ownership WHERE scope=$1 AND task_name=$2 AND task_id=$3`, scope, lockName, taskID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: task ownership could not be read", ErrFenced)
	}
	if err == nil {
		if owner.State == "completed" && !delivery {
			return nil
		}
		if (owner.StoreID != storeID && owner.State != "completed") || owner.State == "running" {
			return fmt.Errorf("%w: task has unresolved execution ownership", ErrFenced)
		}
	}
	_, err = connection.ExecContext(ctx, `INSERT INTO convoy.queue_execution_ownership(scope,task_name,task_id,store_id,state) VALUES($1,$2,$3,$4,'running')
 ON CONFLICT(scope,task_name,task_id) DO UPDATE SET store_id=EXCLUDED.store_id,state='running',updated_at=clock_timestamp()`, scope, lockName, taskID, storeID)
	if err != nil {
		return fmt.Errorf("%w: task ownership could not be persisted", ErrFenced)
	}
	workErr := run(queue.WithAuthoritativeReads(ctx))
	if errors.Is(workErr, ErrEvidence) {
		// An executor could not reconcile its external effect with persistence.
		// Keep running ownership so neither task name nor store can replay it.
		return workErr
	}
	state := "completed"
	if workErr != nil {
		state = "retry"
	}
	settle, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = connection.ExecContext(settle, `UPDATE convoy.queue_execution_ownership SET state=$4,updated_at=clock_timestamp() WHERE scope=$1 AND task_name=$2 AND task_id=$3`, scope, lockName, taskID, state)
	if workErr != nil {
		return workErr
	}
	if err != nil {
		return fmt.Errorf("%w: task outcome could not be persisted", ErrFenced)
	}
	return nil
}
