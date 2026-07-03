package license

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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

// trialCounterTTL lets Redis reclaim the per-day counter; the UTC-day key already
// rolls the count at midnight.
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

// TrialEventLimiter resolves an org's daily event cap from its encrypted
// license_data and enforces it with a per-org, per-UTC-day Redis counter. It is
// cloud-only; callers gate on config.UsesOrgBilling before invoking Allow.
type TrialEventLimiter struct {
	redis  redis.UniversalClient
	logger log.Logger

	mu    sync.Mutex
	cache map[string]cachedTrialLimit
}

// NewTrialEventLimiter builds a limiter over the given Redis client. A nil client
// yields a limiter whose Allow is a no-op (no cap enforced).
func NewTrialEventLimiter(r redis.UniversalClient, logger log.Logger) *TrialEventLimiter {
	return &TrialEventLimiter{
		redis:  r,
		logger: logger,
		cache:  make(map[string]cachedTrialLimit),
	}
}

// Allow returns ErrDailyEventLimit when the org's trial daily cap is reached, or
// nil otherwise. Orgs with no cap (paid, self-hosted, unreadable license_data)
// always pass. Fails open on a Redis error: a cost cap must not hard-block
// ingestion during an outage.
func (t *TrialEventLimiter) Allow(ctx context.Context, orgID, licenseData string) error {
	if t == nil || t.redis == nil {
		return nil
	}

	limit := t.resolveLimit(orgID, licenseData)
	if limit <= 0 {
		return nil
	}

	key := trialDailyKey(orgID, time.Now().UTC())
	res, err := trialEventCounterScript.Run(ctx, t.redis, []string{key}, limit, int64(trialCounterTTL.Seconds())).Int64()
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("trial event limiter: redis error, allowing event (fail-open)", "error", err, "org_id", orgID)
		}
		return nil
	}
	if res == -1 {
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
