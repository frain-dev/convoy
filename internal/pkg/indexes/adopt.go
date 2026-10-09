package indexes

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// adoptLockTimeout bounds the ACCESS EXCLUSIVE wait drop_invalid_index takes.
// Boot calls Adopt synchronously before the listener starts, so an unbounded
// wait behind a long-running query on the same table is a hung startup rather
// than a slow one. The migration that introduced the function bounds it the
// same way.
const adoptLockTimeout = "2s"

// Adopt drops every invalid heap index the catalog still holds that nothing has
// taken responsibility for yet, recording each in dropped_indexes the way a
// migration would. Boot runs this before the rebuild walker so indexes that
// turned invalid after the last migrate are not invisible until the next upgrade.
//
// Indexes reported as busy are left alone: a build in progress is also marked
// invalid until it finishes, and dropping it would destroy work someone is
// waiting on. An index whose table is too busy to lock right now is left the same
// way, since the next boot finds it again.
func Adopt(ctx context.Context, db *pgxpool.Pool) (int, error) {
	invalid, err := ListInvalid(ctx, db)
	if err != nil {
		return 0, err
	}

	var adopted int
	for _, i := range invalid {
		if i.Busy {
			continue
		}
		dropped, err := adoptOne(ctx, db, i.Name)
		if err != nil {
			return adopted, fmt.Errorf("adopting invalid index %s: %w", i.Name, err)
		}
		if dropped {
			adopted++
		}
	}
	return adopted, nil
}

// adoptOne runs one drop with a bounded lock wait. SET LOCAL keeps the timeout
// on the transaction rather than handing it back to the pool, which the plain
// function call allows because nothing here is CONCURRENTLY.
func adoptOne(ctx context.Context, db *pgxpool.Pool, name string) (bool, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '`+adoptLockTimeout+`'`); err != nil {
		return false, fmt.Errorf("bounding the drop lock wait: %w", err)
	}

	var dropped bool
	if err := tx.QueryRow(ctx, `SELECT convoy.drop_invalid_index($1)`, name).Scan(&dropped); err != nil {
		// A lock we could not take says the table is busy, which is the same
		// answer as Busy above: leave the index for the next boot rather than
		// abandoning the indexes after it in the list.
		if isLockTimeout(err) {
			return false, nil
		}
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return dropped, nil
}
