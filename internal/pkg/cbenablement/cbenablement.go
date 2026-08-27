// Package cbenablement resolves whether circuit breaking is enabled, as a single
// source of truth shared by the sampler, enforcement and the dashboard display.
//
// Semantics:
//   - admin_managed=false: env is the instance default.
//   - admin_managed=true: the DB instance flag is the instance default.
//   - A per-org override always wins over instanceBase, including a disabled override.
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
	fflag        *fflag.FFlag
	fetcher      fflag.FeatureFlagFetcher
	adminManaged bool
	clock        clock.Clock
	logger       log.Logger
	ttl          time.Duration

	mu       sync.Mutex
	orgCache map[string]cacheEntry
	anyCache *cacheEntry
}

// NewResolver builds a resolver. adminManaged is read once at process start.
func NewResolver(f *fflag.FFlag, fetcher fflag.FeatureFlagFetcher, adminManaged bool, c clock.Clock, logger log.Logger) *Resolver {
	return &Resolver{
		fflag:        f,
		fetcher:      fetcher,
		adminManaged: adminManaged,
		clock:        c,
		logger:       logger,
		ttl:          defaultTTL,
		orgCache:     make(map[string]cacheEntry),
	}
}

// EnabledForOrg reports whether circuit breaking is enabled for the given org:
// override wins, else the instance default selected by adminManaged. TTL-cached per org.
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
// instance (selected instance default OR any enabled org override). Used to gate the sampler.
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
	return EnabledForOrg(ctx, r.fflag, r.fetcher, r.adminManaged, orgID)
}

// EnabledForOrg resolves per-org circuit-breaker enablement without caching, for
// callers off the hot path (e.g. the dashboard display gate and the org feature-flag
// map). Same semantics as Resolver.EnabledForOrg: a per-org override always wins
// (including a disabled one); otherwise the selected instance default applies.
func EnabledForOrg(ctx context.Context, f *fflag.FFlag, fetcher fflag.FeatureFlagFetcher, adminManaged bool, orgID string) bool {
	envOn := f != nil && f.CanAccessFeature(fflag.CircuitBreaker)

	if fetcher == nil {
		return !adminManaged && envOn
	}

	info, err := fetcher.FetchFeatureFlag(ctx, string(fflag.CircuitBreaker))
	if err != nil || info == nil {
		return !adminManaged && envOn
	}

	// A per-org override always wins, including a disabled one. We mirror the
	// existing CanAccessOrgFeature behavior and treat an override fetch error as
	// "no override" (the common case is no override at all).
	if override, ovErr := fetcher.FetchFeatureFlagOverride(ctx, orgOwnerType, orgID, info.UID); ovErr == nil && override != nil {
		return override.Enabled
	}

	if adminManaged {
		return info.Enabled
	}
	return envOn
}

func (r *Resolver) resolveAnywhere(ctx context.Context) bool {
	if !r.adminManaged && r.envOn() {
		return true
	}
	if r.fetcher == nil {
		return false
	}

	info, err := r.fetcher.FetchFeatureFlag(ctx, string(fflag.CircuitBreaker))
	if err != nil || info == nil {
		// Env mode is already false; Admin mode cannot read its source.
		return false
	}
	if r.adminManaged && info.Enabled {
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
