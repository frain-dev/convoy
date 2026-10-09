package circuit_breaker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/frain-dev/convoy/internal/pkg/pglock"
)

// PostgresStore keeps breaker state in convoy.kv_cache. Lock uses a
// session advisory lock because the sampler holds it across the sample
// window sleep.
type PostgresStore struct {
	db *sqlx.DB
}

func NewPostgresStore(db *sqlx.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Lock(ctx context.Context, mutexKey string, _ uint64) (Locker, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	mu, err := pglock.TryLock(ctx, s.db, mutexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain lock: %v", err)
	}
	return mu, nil
}

func (s *PostgresStore) Unlock(ctx context.Context, mutex Locker) error {
	if mutex == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return mutex.Unlock(ctx)
}

func (s *PostgresStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	err := s.db.SelectContext(ctx, &keys, `
		SELECT key
		FROM convoy.kv_cache
		WHERE key LIKE $1 || '%'
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		pattern,
	)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *PostgresStore) GetOne(ctx context.Context, key string) (string, error) {
	var raw []byte
	err := s.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrCircuitBreakerNotFound
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *PostgresStore) GetMany(ctx context.Context, keys ...string) ([]any, error) {
	if len(keys) == 0 {
		return []any{}, nil
	}

	type row struct {
		Key   string `db:"key"`
		Value []byte `db:"value"`
	}
	var rows []row
	err := s.db.SelectContext(ctx, &rows, `
		SELECT key, value
		FROM convoy.kv_cache
		WHERE key = ANY($1)
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		pq.Array(keys),
	)
	if err != nil {
		return nil, err
	}

	found := make(map[string]string, len(rows))
	for _, r := range rows {
		found[r.Key] = string(r.Value)
	}

	out := make([]any, len(keys))
	for i, key := range keys {
		if v, ok := found[key]; ok {
			out[i] = v
		}
	}
	return out, nil
}

func (s *PostgresStore) SetOne(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var raw []byte
	switch v := value.(type) {
	case CircuitBreaker:
		raw = []byte(v.String())
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		raw = []byte(fmt.Sprint(value))
	}

	var expiresAt interface{}
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO convoy.kv_cache (key, value, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at`,
		key, raw, expiresAt,
	)
	return err
}

func (s *PostgresStore) SetMany(ctx context.Context, breakers map[string]CircuitBreaker, ttl time.Duration) error {
	for key, breaker := range breakers {
		if err := s.SetOne(ctx, key, breaker.String(), ttl); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM convoy.kv_cache WHERE key = $1`, key)
	return err
}
