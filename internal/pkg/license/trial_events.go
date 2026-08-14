package license

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// trialDailyEventLimitKey is the numeric entitlement the billing service injects for an
// active cloud trial. Absent for paid and self-hosted, which have no cap.
const trialDailyEventLimitKey = "daily_event_limit"

// trialLimitCacheTTL bounds in-process memoisation of a resolved cap: short so a
// trial->paid transition is picked up quickly, long enough to spare the decrypt.
const trialLimitCacheTTL = 60 * time.Second

// trialCounterTTL lets Redis reclaim the per-day counter; the UTC-day key
// already rolls the count at midnight. Postgres keeps one row per org and
// replaces its count when the UTC day changes.
const trialCounterTTL = 48 * time.Hour

// ErrDailyEventLimit is returned by Allow when the trial daily cap is reached.
// Callers map it to HTTP 429.
var ErrDailyEventLimit = errors.New("daily trial event limit reached")

// trialEventCounterScript increments the per-org UTC-day counter only while under
// the cap, so requests rejected at the cap never inflate it. Returns the new
// count, or -1 when already at/over the cap.
//
//	KEYS[1]=counter  ARGV[1]=cap  ARGV[2]=ttl seconds
var trialEventCounterScript = redis.NewScript(`
local cap = tonumber(ARGV[1])
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
if current >= cap then
    return -1
end
local n = redis.call("INCR", KEYS[1])
if n == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return n
`)

type cachedTrialLimit struct {
	licenseData string
	limit       int64
	expiresAt   time.Time
}

type trialEventCounter interface {
	Increment(ctx context.Context, orgID string, day time.Time, limit int64) (bool, error)
}

type redisTrialEventCounter struct {
	client redis.UniversalClient
}

func (c *redisTrialEventCounter) Increment(ctx context.Context, orgID string, day time.Time, limit int64) (bool, error) {
	key := trialDailyKey(orgID, day)
	res, err := trialEventCounterScript.Run(ctx, c.client, []string{key}, limit, int64(trialCounterTTL.Seconds())).Int64()
	if err != nil {
		return false, err
	}
	return res != -1, nil
}

type postgresTrialEventCounter struct {
	db *sqlx.DB
}

func (c *postgresTrialEventCounter) Increment(ctx context.Context, orgID string, day time.Time, limit int64) (bool, error) {
	var count int64
	err := c.db.QueryRowContext(ctx, `
		INSERT INTO convoy.trial_event_counters (org_id, day, event_count, updated_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			day = EXCLUDED.day,
			event_count = CASE
				WHEN convoy.trial_event_counters.day = EXCLUDED.day
				THEN convoy.trial_event_counters.event_count + 1
				ELSE 1
			END,
			updated_at = NOW()
		WHERE convoy.trial_event_counters.day <> EXCLUDED.day
		   OR convoy.trial_event_counters.event_count < $3
		RETURNING event_count`,
		orgID, day.UTC().Format("2006-01-02"), limit,
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return count <= limit, nil
}

// TrialEventLimiter resolves an org's daily event cap from its encrypted
// license_data and enforces it with a per-org, per-UTC-day broker counter. It is
// cloud-only; callers gate on config.UsesOrgBilling before invoking Allow.
type TrialEventLimiter struct {
	counter trialEventCounter
	logger  log.Logger

	mu    sync.Mutex
	cache map[string]cachedTrialLimit
}

// NewTrialEventLimiter builds a limiter over Redis. A nil client yields a
// limiter whose Allow is a no-op (no cap enforced).
func NewTrialEventLimiter(r redis.UniversalClient, logger log.Logger) *TrialEventLimiter {
	var counter trialEventCounter
	if r != nil {
		counter = &redisTrialEventCounter{client: r}
	}
	return &TrialEventLimiter{
		counter: counter,
		logger:  logger,
		cache:   make(map[string]cachedTrialLimit),
	}
}

func NewPostgresTrialEventLimiter(db *sqlx.DB, logger log.Logger) *TrialEventLimiter {
	var counter trialEventCounter
	if db != nil {
		counter = &postgresTrialEventCounter{db: db}
	}
	return &TrialEventLimiter{
		counter: counter,
		logger:  logger,
		cache:   make(map[string]cachedTrialLimit),
	}
}

// Allow returns ErrDailyEventLimit when the org's trial daily cap is reached, or
// nil otherwise. Orgs with no cap (paid, self-hosted, unreadable license_data)
// always pass. Fails open on a counter error: a cost cap must not hard-block
// ingestion during an outage.
func (t *TrialEventLimiter) Allow(ctx context.Context, orgID, licenseData string) error {
	if t == nil || t.counter == nil {
		return nil
	}

	limit := t.resolveLimit(orgID, licenseData)
	if limit <= 0 {
		return nil
	}

	allowed, err := t.counter.Increment(ctx, orgID, time.Now().UTC(), limit)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("trial event limiter: counter error, allowing event (fail-open)", "error", err, "org_id", orgID)
		}
		return nil
	}
	if !allowed {
		return ErrDailyEventLimit
	}
	return nil
}

// resolveLimit returns the org's daily cap, memoised for trialLimitCacheTTL. The
// cache is keyed on both org and the exact license_data, so a trial start or a
// trial->paid conversion (both rewrite license_data) invalidates the entry at
// once instead of serving a stale cap. A resolved "no cap" (0) is cached too so
// paid orgs skip the decrypt.
func (t *TrialEventLimiter) resolveLimit(orgID, licenseData string) int64 {
	t.mu.Lock()
	if c, ok := t.cache[orgID]; ok && c.licenseData == licenseData && time.Now().Before(c.expiresAt) {
		t.mu.Unlock()
		return c.limit
	}
	t.mu.Unlock()

	limit := computeDailyEventLimit(orgID, licenseData, t.logger)

	t.mu.Lock()
	t.cache[orgID] = cachedTrialLimit{licenseData: licenseData, limit: limit, expiresAt: time.Now().Add(trialLimitCacheTTL)}
	t.mu.Unlock()

	return limit
}

// DailyEventLimit returns the org's trial daily event cap derived from its encrypted
// license_data, or 0 when there is no cap. Exported so callers (e.g. trial activation)
// can detect once a trial's daily_event_limit entitlement has propagated into license_data.
func DailyEventLimit(orgID, licenseData string) int64 {
	return computeDailyEventLimit(orgID, licenseData, nil)
}

func EntitlementsHaveDailyEventLimit(entitlements map[string]interface{}) bool {
	if len(entitlements) == 0 {
		return false
	}
	parsed := licensesvc.ParseEntitlements(entitlements)
	_, ok := licensesvc.GetNumberEntitlement(parsed, trialDailyEventLimitKey)
	return ok
}

// computeDailyEventLimit decrypts license_data and reads daily_event_limit.
// Returns 0 (no cap) for empty/unreadable data or an absent/non-positive value.
func computeDailyEventLimit(orgID, licenseData string, logger log.Logger) int64 {
	if licenseData == "" {
		return 0
	}

	payload, err := DecryptLicenseData(orgID, licenseData)
	if err != nil {
		if logger != nil {
			logger.Warn("trial event limiter: decrypt license data failed", "error", err, "org_id", orgID)
		}
		return 0
	}
	if payload == nil || len(payload.Entitlements) == 0 {
		return 0
	}

	entitlements := licensesvc.ParseEntitlements(payload.Entitlements)
	limit, ok := licensesvc.GetNumberEntitlement(entitlements, trialDailyEventLimitKey)
	if !ok || limit <= 0 {
		return 0
	}
	return limit
}

func trialDailyKey(orgID string, now time.Time) string {
	return fmt.Sprintf("trial_daily_events:%s:%s", orgID, now.Format("20060102"))
}
