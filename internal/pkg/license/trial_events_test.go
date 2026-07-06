package license

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func encryptWith(t *testing.T, orgID string, entitlements map[string]interface{}) string {
	t.Helper()
	enc, err := EncryptLicenseData(orgID, &LicenseDataPayload{Key: "lk", Entitlements: entitlements})
	require.NoError(t, err)
	return enc
}

func TestComputeDailyEventLimit(t *testing.T) {
	orgID := "org-compute-1"

	tests := []struct {
		name        string
		licenseData string
		want        int64
	}{
		{
			name:        "empty license data yields no cap",
			licenseData: "",
			want:        0,
		},
		{
			name:        "invalid ciphertext yields no cap",
			licenseData: "not-a-valid-ciphertext",
			want:        0,
		},
		{
			name:        "daily_event_limit present is returned",
			licenseData: encryptWith(t, orgID, map[string]interface{}{"daily_event_limit": 100, "rbac": true}),
			want:        100,
		},
		{
			name:        "daily_event_limit absent yields no cap",
			licenseData: encryptWith(t, orgID, map[string]interface{}{"project_limit": 1, "rbac": true}),
			want:        0,
		},
		{
			name:        "non-positive daily_event_limit yields no cap",
			licenseData: encryptWith(t, orgID, map[string]interface{}{"daily_event_limit": 0}),
			want:        0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeDailyEventLimit(orgID, tc.licenseData, nil)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEntitlementsHaveDailyEventLimit(t *testing.T) {
	tests := []struct {
		name         string
		entitlements map[string]interface{}
		want         bool
	}{
		{name: "nil map", entitlements: nil, want: false},
		{name: "empty map", entitlements: map[string]interface{}{}, want: false},
		{
			name:         "present positive value",
			entitlements: map[string]interface{}{"daily_event_limit": 100, "project_limit": 1},
			want:         true,
		},
		{
			name:         "present unlimited (-1) still counts",
			entitlements: map[string]interface{}{"daily_event_limit": -1},
			want:         true,
		},
		{
			name:         "absent key (paid/self-hosted or billing fail-open)",
			entitlements: map[string]interface{}{"project_limit": 1, "rbac": true},
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, EntitlementsHaveDailyEventLimit(tc.entitlements))
		})
	}
}

func TestComputeDailyEventLimit_WrongOrgKeyFailsToDecrypt(t *testing.T) {
	enc := encryptWith(t, "org-a", map[string]interface{}{"daily_event_limit": 100})
	// A different org id derives a different AES key, so decrypt fails -> no cap.
	require.Equal(t, int64(0), computeDailyEventLimit("org-b", enc, nil))
}

func TestTrialDailyKey_RollsOverPerUTCDay(t *testing.T) {
	day1 := time.Date(2026, 7, 1, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 2, 0, 1, 0, 0, time.UTC)

	require.Equal(t, "trial_daily_events:org-1:20260701", trialDailyKey("org-1", day1))
	require.Equal(t, "trial_daily_events:org-1:20260702", trialDailyKey("org-1", day2))
	require.NotEqual(t, trialDailyKey("org-1", day1), trialDailyKey("org-1", day2))
}

func TestTrialEventLimiter_NilRedisAllows(t *testing.T) {
	l := NewTrialEventLimiter(nil, nil)
	require.NoError(t, l.Allow(context.Background(), "org-1", ""))
}

func TestTrialEventLimiter_RedisDownFailsOpen(t *testing.T) {
	// Point at a closed port with a short dial timeout so the script errors fast.
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 150 * time.Millisecond,
		ReadTimeout: 150 * time.Millisecond,
	})
	defer client.Close()

	l := NewTrialEventLimiter(client, nil)
	orgID := ulid.Make().String()
	// Seed a real cap so we exercise the counter path, not the no-cap shortcut.
	l.cache[orgID] = cachedTrialLimit{limit: 1, expiresAt: time.Now().Add(time.Hour)}

	// Both calls must be allowed (fail-open) despite Redis being unreachable.
	require.NoError(t, l.Allow(context.Background(), orgID, ""))
	require.NoError(t, l.Allow(context.Background(), orgID, ""))
}

func newTestRedis(t *testing.T) redis.UniversalClient {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DialTimeout: 300 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis not available on localhost:6379: %v", err)
	}
	return client
}

func TestTrialEventLimiter_Allow_UnderAtOverCap(t *testing.T) {
	client := newTestRedis(t)
	defer client.Close()

	l := NewTrialEventLimiter(client, nil)
	orgID := ulid.Make().String()
	l.cache[orgID] = cachedTrialLimit{limit: 3, expiresAt: time.Now().Add(time.Hour)}

	ctx := context.Background()
	key := trialDailyKey(orgID, time.Now().UTC())
	defer client.Del(ctx, key)

	// First 3 events accepted.
	for i := 0; i < 3; i++ {
		require.NoError(t, l.Allow(ctx, orgID, ""), "event %d should be allowed", i+1)
	}

	// 4th and 5th rejected with ErrDailyEventLimit; the counter must not keep
	// inflating past the cap.
	require.ErrorIs(t, l.Allow(ctx, orgID, ""), ErrDailyEventLimit)
	require.ErrorIs(t, l.Allow(ctx, orgID, ""), ErrDailyEventLimit)

	count, err := client.Get(ctx, key).Int64()
	require.NoError(t, err)
	require.Equal(t, int64(3), count, "blocked requests should not inflate the counter")
}

func TestTrialEventLimiter_Allow_NoCapWhenLimitZero(t *testing.T) {
	client := newTestRedis(t)
	defer client.Close()

	l := NewTrialEventLimiter(client, nil)
	orgID := ulid.Make().String()
	// No cap resolved (paid/self-hosted): every event allowed, no counter key set.
	l.cache[orgID] = cachedTrialLimit{limit: 0, expiresAt: time.Now().Add(time.Hour)}

	ctx := context.Background()
	key := trialDailyKey(orgID, time.Now().UTC())
	defer client.Del(ctx, key)

	for i := 0; i < 10; i++ {
		require.NoError(t, l.Allow(ctx, orgID, ""))
	}

	exists, err := client.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists, "no-cap orgs must not create a counter key")
}
