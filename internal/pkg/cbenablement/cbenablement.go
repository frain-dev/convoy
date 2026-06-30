// Package cbenablement resolves whether circuit breaking is enabled, as a single
// source of truth shared by the sampler, enforcement and the dashboard display.
//
// Semantics (mode-safe across cloud, licensed self-hosted, and unlicensed self-hosted):
//   - instanceBase = env (CONVOY_ENABLE_FEATURE_FLAG, static) OR the DB instance flag (live).
//     The env flag is folded into the instance default, not a blanket per-org force-on.
//   - A per-org override always wins over instanceBase, including a disabled override.
//     This preserves an operator's per-org scoping even when the platform sets the env flag.
//   - The global sampler must run when enabled anywhere: instanceBase OR any enabled override.
//
// Reads are cached with a short TTL and refreshed lazily on read, so toggling the
// instance flag or an org override takes effect live without restarting the worker
// and without a background goroutine.
package cbenablement

import (
	"context"
	"sync"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const orgOwnerType = "organisation"

// defaultTTL bounds how stale a resolved value can be. Short enough to feel live,
// long enough to keep the per-delivery enforcement path off the DB on every call.
const defaultTTL = 15 * time.Second

type cacheEntry struct {
	enabled  bool
	expireAt time.Time
}

// Resolver resolves circuit-breaker enablement with a TTL cache.
type Resolver struct {
	fflag   *fflag.FFlag
	fetcher fflag.FeatureFlagFetcher
	clock   clock.Clock
	logger  log.Logger
	ttl     time.Duration

	mu       sync.Mutex
	orgCache map[string]cacheEntry
	anyCache *cacheEntry
}

// NewResolver builds a resolver. fetcher and logger must not be nil in production;
// fflag carries the static env state.
func NewResolver(f *fflag.FFlag, fetcher fflag.FeatureFlagFetcher, c clock.Clock, logger log.Logger) *Resolver {
	return &Resolver{
		fflag:    f,
		fetcher:  fetcher,
		clock:    c,
		logger:   logger,
		ttl:      defaultTTL,
		orgCache: make(map[string]cacheEntry),
	}
}

// EnabledForOrg reports whether circuit breaking is enabled for the given org:
// override wins, else instanceBase (env OR instance DB flag). TTL-cached per org.
func (r *Resolver) EnabledForOrg(ctx context.Context, orgID string) bool {
	now := r.clock.Now()

	r.mu.Lock()
	if e, ok := r.orgCache[orgID]; ok && now.Before(e.expireAt) {
		r.mu.Unlock()
		return e.enabled
	}
	r.mu.Unlock()

	enabled := r.resolveForOrg(ctx, orgID)

	r.mu.Lock()
	r.orgCache[orgID] = cacheEntry{enabled: enabled, expireAt: now.Add(r.ttl)}
	r.mu.Unlock()

	return enabled
}

// EnabledAnywhere reports whether circuit breaking is enabled anywhere on the
// instance (instanceBase OR any enabled org override). Used to gate the sampler.
func (r *Resolver) EnabledAnywhere(ctx context.Context) bool {
	now := r.clock.Now()

	r.mu.Lock()
	if r.anyCache != nil && now.Before(r.anyCache.expireAt) {
		v := r.anyCache.enabled
		r.mu.Unlock()
		return v
	}
	r.mu.Unlock()

	enabled := r.resolveAnywhere(ctx)

	r.mu.Lock()
	r.anyCache = &cacheEntry{enabled: enabled, expireAt: now.Add(r.ttl)}
	r.mu.Unlock()

	return enabled
}

func (r *Resolver) envOn() bool {
	if r.fflag == nil {
		return false
	}
	return r.fflag.CanAccessFeature(fflag.CircuitBreaker)
}

func (r *Resolver) resolveForOrg(ctx context.Context, orgID string) bool {
	return EnabledForOrg(ctx, r.fflag, r.fetcher, orgID)
}

// EnabledForOrg resolves per-org circuit-breaker enablement without caching, for
// callers off the hot path (e.g. the dashboard display gate and the org feature-flag
// map). Same semantics as Resolver.EnabledForOrg: a per-org override always wins
// (including a disabled one); otherwise the instance base = env OR the DB instance flag.
func EnabledForOrg(ctx context.Context, f *fflag.FFlag, fetcher fflag.FeatureFlagFetcher, orgID string) bool {
	envOn := f != nil && f.CanAccessFeature(fflag.CircuitBreaker)

	if fetcher == nil {
		return envOn
	}

	info, err := fetcher.FetchFeatureFlag(ctx, string(fflag.CircuitBreaker))
	if err != nil || info == nil {
		// Flag row missing or DB error: the instance base reduces to the env flag.
		// Fail closed beyond env (a flaky read does not flip behavior on by itself).
		return envOn
	}

	// A per-org override always wins, including a disabled one. We mirror the
	// existing CanAccessOrgFeature behavior and treat an override fetch error as
	// "no override" (the common case is no override at all).
	if override, ovErr := fetcher.FetchFeatureFlagOverride(ctx, orgOwnerType, orgID, info.UID); ovErr == nil && override != nil {
		return override.Enabled
	}

	// No override: instance base = env OR the DB instance flag.
	return envOn || info.Enabled
}

func (r *Resolver) resolveAnywhere(ctx context.Context) bool {
	if r.envOn() {
		// env is the instance-wide default; if set, the sampler must run.
		return true
	}

	info, err := r.fetcher.FetchFeatureFlag(ctx, string(fflag.CircuitBreaker))
	if err != nil || info == nil {
		// env already false and the instance flag is unreadable: fail closed.
		return false
	}
	if info.Enabled {
		return true
	}

	// Instance base is off; the sampler still runs if any org override enables it.
	hasEnabledOverride, err := r.fetcher.AnyEnabledOverride(ctx, info.UID)
	if err != nil {
		if r.logger != nil {
			r.logger.Warnf("[circuit breaker] failed to check enabled overrides, treating as disabled: %v", err)
		}
		return false
	}

	return hasEnabledOverride
}
