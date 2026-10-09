package usage

import (
	"context"
	"testing"
	"time"

	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
)

type usageCache struct {
	snapshot licenseservice.UsageSnapshot
	ttl      time.Duration
}

func (c *usageCache) Set(_ context.Context, _ string, data interface{}, ttl time.Duration) error {
	c.snapshot = *data.(*licenseservice.UsageSnapshot)
	c.ttl = ttl
	return nil
}

func (c *usageCache) Get(_ context.Context, _ string, dest interface{}) error {
	*dest.(*licenseservice.UsageSnapshot) = c.snapshot
	return nil
}

func (c *usageCache) Delete(context.Context, string) error {
	return nil
}

func TestLoadCachedNilCacheOmitsUsage(t *testing.T) {
	s := NewStore(nil, nil)
	snap, err := s.LoadCached(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap != nil {
		t.Fatalf("expected nil snapshot without cache, got %+v", snap)
	}
}

func TestSaveNilCacheIsNoop(t *testing.T) {
	s := NewStore(nil, nil)
	err := s.Save(context.Background(), &licenseservice.UsageSnapshot{
		EndpointCount: 1,
		EventCount:    2,
		ProjectCount:  3,
		OrgCount:      4,
		UserCount:     5,
		AsOf:          time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("Save without cache should be noop, got %v", err)
	}
}

func TestSaveAndLoadCachedRoundTrip(t *testing.T) {
	c := &usageCache{}
	s := NewStore(nil, c)
	want := &licenseservice.UsageSnapshot{
		EndpointCount: 1,
		EventCount:    2,
		ProjectCount:  3,
		OrgCount:      4,
		UserCount:     5,
		AsOf:          time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.Save(context.Background(), want); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	got, err := s.LoadCached(context.Background())
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if *got != *want {
		t.Fatalf("snapshot mismatch: got %+v want %+v", got, want)
	}
	if c.ttl != cacheTTL {
		t.Fatalf("cache ttl mismatch: got %v want %v", c.ttl, cacheTTL)
	}
}

func TestRefreshWithoutDBErrors(t *testing.T) {
	s := NewStore(nil, nil)
	_, err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
}
