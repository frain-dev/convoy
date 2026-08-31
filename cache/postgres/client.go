package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jmoiron/sqlx"
	"github.com/vmihailenco/msgpack/v5"
)

// PostgresCache stores msgpack values in convoy.kv_cache, matching Redis
// (go-redis/cache/v9 uses vmihailenco/msgpack without JSON struct tags).
//
// Unlike Redis, this cache lives on the same instance serving the queue and the
// application, so every read competes for the connection pool it is supposed to
// be protecting. local holds a bounded, short-lived copy of recently read values
// to keep repeated reads of the same key off the database.
//
// Only Get consults local. GetStrict, Consume and GetOrCreateBytes are
// authoritative by contract and always read the table. Any method that changes a
// row drops the local copy of that key, so a writer never reads back its own
// stale value; other replicas converge within the entry's lifetime.
type PostgresCache struct {
	db    *sqlx.DB
	local *expirable.LRU[string, []byte]

	// writes counts invalidations per stripe. Get samples a key's stripe before
	// it reads the table and again before it caches, and skips caching if the
	// two differ, so a Delete or Set that lands mid-read cannot be undone by the
	// reader storing the value it fetched just before the write. Stripes rather
	// than per-key counters keep this fixed-size and lock-free; a collision only
	// costs an unrelated key one skipped cache fill.
	writes [invalidationStripes]atomic.Uint64
}

// invalidationStripes is a power of two so the index is a mask.
const invalidationStripes = 1024

func (c *PostgresCache) stripeFor(key string) *atomic.Uint64 {
	// FNV-1a, inline to avoid allocating a hasher on every read.
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return &c.writes[h&(invalidationStripes-1)]
}

func New(db *sqlx.DB) *PostgresCache {
	return &PostgresCache{db: db}
}

// NewWithLocalReads returns a cache that may serve Get from process memory for
// up to ttl, holding at most size keys. A non-positive ttl or size disables it
// and every read goes to the table.
//
// The tradeoff this buys and costs: an invalidation is immediate on the replica
// that performed it and takes up to ttl to be observed elsewhere. That is bounded
// by the same entries already being cached for minutes at a time, so a ttl of a
// second or two sits well inside the staleness the callers accept, while removing
// nearly all of the repeated reads that a busy instance would otherwise issue for
// the same handful of keys.
func NewWithLocalReads(db *sqlx.DB, ttl time.Duration, size int) *PostgresCache {
	if ttl <= 0 || size <= 0 {
		return New(db)
	}
	return &PostgresCache{
		db:    db,
		local: expirable.NewLRU[string, []byte](size, nil, ttl),
	}
}

// forget drops the local copy of a key. Called by every method that changes the
// stored row so a writer cannot read back what it just replaced.
func (c *PostgresCache) forget(key string) {
	if c.local != nil {
		// Bump first: a Get that sampled the stripe before this Add and stores
		// after it will skip, whether or not Remove has run yet. The reverse
		// order leaves a window between Remove and Add where that Get sees an
		// unchanged stripe, stores the row it already read, and resurrects a
		// deleted value for the rest of the entry's lifetime.
		c.stripeFor(key).Add(1)
		c.local.Remove(key)
	}
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
	c.forget(key)
	return err
}

func (c *PostgresCache) Get(ctx context.Context, key string, dest interface{}) error {
	var stripe *atomic.Uint64
	var before uint64

	if c.local != nil {
		if raw, ok := c.local.Get(key); ok {
			// Unmarshal per caller rather than sharing a decoded value, so one
			// caller mutating what it got back cannot corrupt another's copy.
			if err := msgpack.Unmarshal(raw, dest); err == nil {
				return nil
			}
			// Bytes that will not decode must not keep answering for the rest
			// of the entry's life. Drop them and let the table decide.
			c.local.Remove(key)
		}
		stripe = c.stripeFor(key)
		before = stripe.Load()
	}

	var raw []byte
	err := c.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Misses are not held locally: the caller is about to populate the key.
		return nil
	}
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(raw, dest); err != nil {
		// decode failure is a miss: fail open to DB rather than a successful empty value
		return nil
	}
	// Cache only what decoded, and only if nothing invalidated this key while
	// the read was in flight. Storing across a concurrent Delete or Set would
	// resurrect the value the writer just removed for the rest of the entry's
	// lifetime, including for the writer's own next read.
	if c.local != nil && stripe.Load() == before {
		c.local.Add(key, raw)
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
	c.forget(key)
	return err
}

// Consume atomically reads and deletes a live cache value. It is intentionally
// separate from Get because one-shot signals must not be observed twice.
func (c *PostgresCache) Consume(ctx context.Context, key string, dest interface{}) (bool, error) {
	// Drop the local copy whatever the outcome: after an attempt to consume, an
	// unknown result must send the next Get to the table rather than answer from
	// memory.
	defer c.forget(key)

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

// GetBytes reads a raw byte value written by GetOrCreateBytes. Miss returns nil, nil.
func (c *PostgresCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	var raw []byte
	err := c.db.GetContext(ctx, &raw, `
		SELECT value
		FROM convoy.kv_cache
		WHERE key = $1
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}

// GetOrCreateBytes inserts value when the key is absent (SETNX). Concurrent
// writers converge on the first row. Used for cluster-wide secrets such as the
// queue-monitoring cookie signing key.
func (c *PostgresCache) GetOrCreateBytes(ctx context.Context, key string, value []byte) ([]byte, error) {
	defer c.forget(key)

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
