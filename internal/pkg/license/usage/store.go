package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
)

const (
	cacheKey = "convoy:usage:snapshot:v2"
	// Must outlive SnapshotUsage (nightly ~02:15). 48h leaves headroom if a
	// cron tick is missed; after expiry LoadCached omits usage (fail open).
	cacheTTL = 48 * time.Hour
)

// Store materializes anonymized instance counts into the active broker cache.
type Store struct {
	db    database.Database
	cache cache.Cache
}

func NewStore(db database.Database, cache cache.Cache) *Store {
	return &Store{db: db, cache: cache}
}

// Refresh runs cheap instance-wide COUNT(*) queries and caches the snapshot.
// Failure policy: callers treat errors as non-fatal (omit usage / skip cron).
func (s *Store) Refresh(ctx context.Context) (*licenseservice.UsageSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("usage store not configured")
	}

	var endpoints, events, projects, orgs, users int64
	queries := []struct {
		dest *int64
		sql  string
	}{
		{&endpoints, `SELECT COUNT(*) FROM convoy.endpoints WHERE deleted_at IS NULL`},
		{&events, `SELECT COUNT(*) FROM convoy.events WHERE deleted_at IS NULL`},
		{&projects, `SELECT COUNT(*) FROM convoy.projects WHERE deleted_at IS NULL`},
		{&orgs, `SELECT COUNT(*) FROM convoy.organisations WHERE deleted_at IS NULL`},
		{&users, `SELECT COUNT(*) FROM convoy.users WHERE deleted_at IS NULL`},
	}
	for _, q := range queries {
		if err := s.db.GetDB().GetContext(ctx, q.dest, q.sql); err != nil {
			return nil, fmt.Errorf("count query failed: %w", err)
		}
	}

	snap := &licenseservice.UsageSnapshot{
		EndpointCount: endpoints,
		EventCount:    events,
		ProjectCount:  projects,
		OrgCount:      orgs,
		UserCount:     users,
		AsOf:          time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.Save(ctx, snap); err != nil {
		return snap, err
	}
	return snap, nil
}

// Save writes the snapshot to the active cache. No-op without a cache.
func (s *Store) Save(ctx context.Context, snap *licenseservice.UsageSnapshot) error {
	if s == nil || s.cache == nil || snap == nil {
		return nil
	}
	return s.cache.Set(ctx, cacheKey, snap, cacheTTL)
}

// LoadCached implements licenseservice.UsageLoader. Returns nil,nil on miss.
func (s *Store) LoadCached(ctx context.Context) (*licenseservice.UsageSnapshot, error) {
	if s == nil || s.cache == nil {
		return nil, nil
	}
	var snap licenseservice.UsageSnapshot
	if err := s.cache.Get(ctx, cacheKey, &snap); err != nil {
		// Cache errors omit optional usage from license validation.
		return nil, nil
	}
	if snap.AsOf == "" {
		return nil, nil
	}
	return &snap, nil
}
