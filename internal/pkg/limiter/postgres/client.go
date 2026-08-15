package postgres

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/internal/pkg/limiter"
)

var _ limiter.RateLimiter = (*PostgresLimiter)(nil)

// PostgresLimiter is a token bucket stored in convoy.rate_limits. Burst equals
// rate, matching the Redis limiter. Failure policy: a DB error fails closed
// (the caller sees the error and must not treat it as allowed).
type PostgresLimiter struct {
	db       *sqlx.DB
	leasesMu sync.Mutex
	leases   map[string]*tokenLease
	now      func() time.Time
}

type tokenLease struct {
	mu        sync.Mutex
	rate      int
	duration  int
	remaining int
	expiresAt time.Time
}

func New(db *sqlx.DB) *PostgresLimiter {
	return &PostgresLimiter{
		db:     db,
		leases: make(map[string]*tokenLease),
		now:    time.Now,
	}
}

func (l *PostgresLimiter) Allow(ctx context.Context, key string, rate int) error {
	return l.AllowWithDuration(ctx, key, rate, 1)
}

func (l *PostgresLimiter) AllowWithDuration(ctx context.Context, key string, rate, duration int) error {
	if rate == 0 || duration == 0 {
		return nil
	}

	lease := l.leaseFor(key)
	lease.mu.Lock()
	defer lease.mu.Unlock()

	if lease.rate != rate || lease.duration != duration {
		// A changed limit owns a new contract. Discarding old reservations can
		// only under-admit; reusing them could exceed the new burst.
		lease.rate = rate
		lease.duration = duration
		lease.remaining = 0
		lease.expiresAt = time.Time{}
	}
	now := l.now()
	if lease.remaining > 0 && now.Before(lease.expiresAt) {
		lease.remaining--
		return nil
	}
	// Once the shared bucket can fully refill, unused local reservations are
	// stale. Discarding them prevents old capacity from stacking with a newly
	// refilled shared burst; this can only under-admit.
	lease.remaining = 0
	lease.expiresAt = time.Time{}

	period := time.Duration(duration) * time.Second
	burst := float64(rate)
	refillPerSec := burst / period.Seconds()
	reservationSize := reservationSizeFor(rate, refillPerSec)

	granted, err := l.reserve(ctx, key, burst, refillPerSec, reservationSize)
	if err == nil {
		lease.remaining = granted - 1
		lease.expiresAt = now.Add(period)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var tokens float64
	var updatedAt time.Time
	err = l.db.QueryRowContext(ctx, `
		SELECT tokens, updated_at
		FROM convoy.rate_limits
		WHERE key = $1`,
		key,
	).Scan(&tokens, &updatedAt)
	if err != nil {
		return err
	}

	elapsed := math.Max(now.Sub(updatedAt).Seconds(), 0)
	available := math.Min(burst, tokens+elapsed*refillPerSec)
	retryAfter := time.Duration((1-available)/refillPerSec*float64(time.Second)) + time.Millisecond
	if retryAfter < time.Millisecond {
		retryAfter = time.Millisecond
	}
	return limiter.NewRateLimitExceeded(retryAfter)
}

// minReservation is the floor on how many tokens one refill claims. Small
// limits keep it, so a low limit spread over several replicas behaves as before.
const minReservation = 32

// reservationSizeFor claims roughly one second of refill per round trip. A flat
// floor makes a high limit pay a shared-bucket round trip every few requests,
// and that round trip happens under the per-key lock, so the limiter and not the
// limit decides throughput. Sizing by refill rate keeps the round trip count
// near constant as the limit grows, while the bucket's own burst still caps what
// a reservation can take, so this cannot over-admit.
func reservationSizeFor(rate int, refillPerSec float64) int {
	size := int(math.Ceil(refillPerSec))
	if size < minReservation {
		size = minReservation
	}
	return min(rate, size)
}

func (l *PostgresLimiter) leaseFor(key string) *tokenLease {
	l.leasesMu.Lock()
	defer l.leasesMu.Unlock()

	lease := l.leases[key]
	if lease == nil {
		lease = &tokenLease{}
		l.leases[key] = lease
	}
	return lease
}

func (l *PostgresLimiter) reserve(ctx context.Context, key string, burst, refillPerSec float64, size int) (int, error) {
	// Reservations are deducted from the shared bucket before local use, so
	// multiple processes cannot over-admit. A crash loses unused tokens and
	// therefore fails closed until the normal refill heals the bucket.
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO convoy.rate_limits (key, tokens, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO NOTHING`,
		key, burst,
	)
	if err != nil {
		return 0, err
	}

	var granted int
	err = l.db.QueryRowContext(ctx, `
		WITH available AS MATERIALIZED (
			SELECT LEAST(
				$2,
				tokens + GREATEST(EXTRACT(EPOCH FROM (NOW() - updated_at)), 0) * $3
			) AS tokens
			FROM convoy.rate_limits
			WHERE key = $1
			FOR UPDATE
		),
		reserved AS (
			UPDATE convoy.rate_limits AS limits
			SET
				tokens = available.tokens - LEAST($4, FLOOR(available.tokens)),
				updated_at = NOW()
			FROM available
			WHERE limits.key = $1
			  AND available.tokens >= 1
			RETURNING LEAST($4, FLOOR(available.tokens))::INTEGER AS granted
		)
		SELECT granted FROM reserved`,
		key, burst, refillPerSec, size,
	).Scan(&granted)
	return granted, err
}
