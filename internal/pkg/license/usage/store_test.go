package usage

import (
	"context"
	"testing"
	"time"

	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
)

func TestLoadCachedNilRedisOmitsUsage(t *testing.T) {
	s := NewStore(nil, nil)
	snap, err := s.LoadCached(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap != nil {
		t.Fatalf("expected nil snapshot without redis, got %+v", snap)
	}
}

func TestSaveNilRedisIsNoop(t *testing.T) {
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
		t.Fatalf("Save without redis should be noop, got %v", err)
	}
}

func TestRefreshWithoutDBErrors(t *testing.T) {
	s := NewStore(nil, nil)
	_, err := s.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
}
