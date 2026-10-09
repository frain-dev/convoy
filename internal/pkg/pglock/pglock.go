package pglock

import (
	"context"
	"database/sql"
	"errors"
	"hash/fnv"

	"github.com/jmoiron/sqlx"
)

// ErrNotObtained is returned when pg_try_advisory_lock is false.
var ErrNotObtained = errors.New("advisory lock not obtained")

// Mutex holds a session-level advisory lock on a dedicated connection.
// Unlock releases the lock and returns the connection to the pool. A
// crashed process drops the connection, which Postgres then unlocks.
type Mutex struct {
	conn *sql.Conn
	key  int64
}

func hashKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64())
}

// TryLock takes a dedicated connection and pg_try_advisory_lock. It does
// not block. Failure policy: not obtained is ErrNotObtained, not a retryable
// transport error.
func TryLock(ctx context.Context, db *sqlx.DB, name string) (*Mutex, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	key := hashKey(name)
	var ok bool
	err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !ok {
		_ = conn.Close()
		return nil, ErrNotObtained
	}

	return &Mutex{conn: conn, key: key}, nil
}

func (m *Mutex) Unlock(ctx context.Context) error {
	if m == nil || m.conn == nil {
		return nil
	}
	defer func() {
		_ = m.conn.Close()
		m.conn = nil
	}()

	var ok bool
	err := m.conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", m.key).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("advisory unlock returned false")
	}
	return nil
}
