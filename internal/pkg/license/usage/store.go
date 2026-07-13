package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

const (
	redisKey = "convoy:usage:snapshot"
	// Must outlive SnapshotUsage (nightly ~02:15). 48h leaves headroom if a
	// cron tick is missed; after expiry LoadCached omits usage (fail open).
	redisTTL = 48 * time.Hour
)

// Store materializes anonymized instance counts into Redis for license validate.
type Store struct {
	db    database.Database
	redis *rdb.Redis
}

func NewStore(db database.Database, redis *rdb.Redis) *Store {
	return &Store{db: db, redis: redis}
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

// Save writes the snapshot to Redis. No-op without redis.
func (s *Store) Save(ctx context.Context, snap *licenseservice.UsageSnapshot) error {
	if s == nil || s.redis == nil || snap == nil {
		return nil
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return s.redis.Client().Set(ctx, redisKey, b, redisTTL).Err()
}

// LoadCached implements licenseservice.UsageLoader. Returns nil,nil on miss.
func (s *Store) LoadCached(ctx context.Context) (*licenseservice.UsageSnapshot, error) {
	if s == nil || s.redis == nil {
		return nil, nil
	}
	b, err := s.redis.Client().Get(ctx, redisKey).Bytes()
	if err != nil {
		// miss or redis error: omit usage (fail open for validate)
		return nil, nil
	}
	var snap licenseservice.UsageSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, nil
	}
	return &snap, nil
}
