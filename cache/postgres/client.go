package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/vmihailenco/msgpack/v5"
)

// PostgresCache stores msgpack values in convoy.kv_cache, matching Redis
// (go-redis/cache/v9 uses vmihailenco/msgpack without JSON struct tags).
type PostgresCache struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *PostgresCache {
	return &PostgresCache{db: db}
}

func (c *PostgresCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	raw, err := msgpack.Marshal(data)
	if err != nil {
		return err
	}

	var expiresAt interface{}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	_, err = c.db.ExecContext(ctx, `
		INSERT INTO convoy.kv_cache (key, value, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at`,
		key, raw, expiresAt,
	)
	return err
}

func (c *PostgresCache) Get(ctx context.Context, key string, dest interface{}) error {
	var raw []byte
	err := c.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(raw, dest); err != nil {
		// decode failure is a miss: fail open to DB rather than a successful empty value
		return nil
	}
	return nil
}

// GetStrict distinguishes corrupt values from misses for authoritative cache
// records such as revocation markers.
func (c *PostgresCache) GetStrict(ctx context.Context, key string, dest interface{}) error {
	var raw []byte
	err := c.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode cache value: %w", err)
	}
	return nil
}

func (c *PostgresCache) Delete(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM convoy.kv_cache WHERE key = $1`, key)
	return err
}

// Consume atomically reads and deletes a live cache value. It is intentionally
// separate from Get because one-shot signals must not be observed twice.
func (c *PostgresCache) Consume(ctx context.Context, key string, dest interface{}) (bool, error) {
	var raw []byte
	err := c.db.GetContext(ctx, &raw, `
		DELETE FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING value`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := msgpack.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("decode consumed cache value: %w", err)
	}
	return true, nil
}

// GetOrCreateBytes inserts value when the key is absent (SETNX). Concurrent
// writers converge on the first row. Used for cluster-wide secrets such as the
// queue-monitoring cookie signing key.
func (c *PostgresCache) GetOrCreateBytes(ctx context.Context, key string, value []byte) ([]byte, error) {
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO convoy.kv_cache (key, value, expires_at)
		VALUES ($1, $2, NULL)
		ON CONFLICT (key) DO NOTHING`,
		key, value,
	)
	if err != nil {
		return nil, err
	}

	var raw []byte
	err = c.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
