package indexes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrBlockedByData means the index cannot be built until duplicate rows are
// removed. Retrying without fixing the data will fail the same way.
var ErrBlockedByData = errors.New("index rebuild blocked by duplicate rows")

// IsBlockedByData reports whether a rebuild error is a duplicate-key failure
// rather than a transient one worth retrying on the next boot.
func IsBlockedByData(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not create unique index") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// MarkBlocked records that a rebuild failed on duplicate data and must not be
// retried at boot until an operator fixes the rows and starts it again.
func MarkBlocked(ctx context.Context, db *pgxpool.Pool, name, reason string) error {
	tag, err := db.Exec(ctx, `
        UPDATE convoy.dropped_indexes
        SET blocked_at = NOW(), blocked_reason = $2
        WHERE index_name = $1 AND rebuilt_at IS NULL`, name, reason)
	if err != nil {
		return fmt.Errorf("recording blocked index %s: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("recording blocked index %s: %w", name, ErrNotDropped)
	}
	return nil
}

// ClearBlocked removes a blocked marker before an operator-requested retry.
func ClearBlocked(ctx context.Context, db *pgxpool.Pool, name string) error {
	_, err := db.Exec(ctx, `
        UPDATE convoy.dropped_indexes
        SET blocked_at = NULL, blocked_reason = NULL
        WHERE index_name = $1 AND rebuilt_at IS NULL`, name)
	if err != nil {
		return fmt.Errorf("clearing blocked index %s: %w", name, err)
	}
	return nil
}
