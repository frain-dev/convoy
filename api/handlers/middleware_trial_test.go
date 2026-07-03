package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func trialTestRedis(t *testing.T) redis.UniversalClient {
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

func TestEnforceTrialEventCap_429OnNPlusOne(t *testing.T) {
	client := trialTestRedis(t)
	defer client.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := ulid.Make().String()
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key:          "lk",
		Entitlements: map[string]interface{}{"daily_event_limit": 2},
	})
	require.NoError(t, err)

	org := &datastore.Organisation{UID: orgID, LicenseData: enc}
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgRepo.EXPECT().FetchOrganisationByID(gomock.Any(), orgID).Return(org, nil).AnyTimes()

	a := &types.APIOptions{
		Cfg:         config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}},
		OrgRepo:     mockOrgRepo,
		TrialEvents: license.NewTrialEventLimiter(client, nil),
	}

	defer client.Del(context.Background(), "trial_daily_events:"+orgID+":"+time.Now().UTC().Format("20060102"))

	do := func() (bool, int) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		blocked := EnforceTrialEventCap(rec, req, a, orgID)
		return blocked, rec.Code
	}

	blocked, _ := do()
	require.False(t, blocked, "1st event under cap")
	blocked, _ = do()
	require.False(t, blocked, "2nd event at cap boundary")

	blocked, code := do()
	require.True(t, blocked, "3rd event over cap must be blocked")
	require.Equal(t, http.StatusTooManyRequests, code)
}

// TestEnforceTrialEventCapForNewEvent_DuplicateDoesNotConsumeQuotaOr429 proves the
// API event-create paths are symmetric with source ingest: the duplicate verdict is
// resolved before quota is consumed, so an idempotent replay at the cap neither 429s
// nor spends quota, while genuinely new events still enforce, and a duplicate-lookup
// error fails toward enforcing (treated as new).
func TestEnforceTrialEventCapForNewEvent_DuplicateDoesNotConsumeQuotaOr429(t *testing.T) {
	client := trialTestRedis(t)
	defer client.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := ulid.Make().String()
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key:          "lk",
		Entitlements: map[string]interface{}{"daily_event_limit": 1},
	})
	require.NoError(t, err)

	org := &datastore.Organisation{UID: orgID, LicenseData: enc}
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgRepo.EXPECT().FetchOrganisationByID(gomock.Any(), orgID).Return(org, nil).AnyTimes()
	mockEventRepo := mocks.NewMockEventRepository(ctrl)

	projectID := "project-trial-dup"
	h := &Handler{A: &types.APIOptions{
		Cfg:         config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}},
		Logger:      log.New("convoy", log.LevelInfo),
		OrgRepo:     mockOrgRepo,
		EventRepo:   mockEventRepo,
		TrialEvents: license.NewTrialEventLimiter(client, nil),
	}}

	defer client.Del(context.Background(), "trial_daily_events:"+orgID+":"+time.Now().UTC().Format("20060102"))

	do := func(key string) (bool, int) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		blocked := h.enforceTrialEventCapForNewEvent(rec, req, orgID, projectID, key, h.duplicateByAnyEvent)
		return blocked, rec.Code
	}

	// First event with a fresh idempotency key is new and consumes the whole cap (limit 1).
	mockEventRepo.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), projectID, "idem-1").Return(false, nil)
	blocked, _ := do("idem-1")
	require.False(t, blocked, "first new event under cap must pass")

	// Idempotent replay at the cap: deduplicated downstream and never delivered, so it
	// must not 429 and must not consume quota.
	mockEventRepo.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), projectID, "idem-1").Return(true, nil)
	blocked, code := do("idem-1")
	require.False(t, blocked, "duplicate replay at the cap must pass")
	require.NotEqual(t, http.StatusTooManyRequests, code)

	// A genuinely new event at the cap is still blocked, proving the replay above
	// consumed nothing.
	mockEventRepo.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), projectID, "idem-2").Return(false, nil)
	blocked, code = do("idem-2")
	require.True(t, blocked, "new event over cap must be blocked")
	require.Equal(t, http.StatusTooManyRequests, code)

	// Duplicate-lookup error: fail toward enforcing (treated as new), so at the cap the
	// request is blocked rather than silently exempted.
	mockEventRepo.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), projectID, "idem-1").Return(false, context.DeadlineExceeded)
	blocked, code = do("idem-1")
	require.True(t, blocked, "lookup error at the cap must still enforce")
	require.Equal(t, http.StatusTooManyRequests, code)

	// No idempotency key: no duplicate lookup (no EXPECT registered for an empty key)
	// and normal enforcement applies.
	blocked, code = do("")
	require.True(t, blocked, "keyless event over cap must be blocked")
	require.Equal(t, http.StatusTooManyRequests, code)
}

// TestEnforceTrialEventCapForNewEvent_PassThroughWhenNotCloud proves the wrapper
// short-circuits before the duplicate lookup on non-cloud instances: the strict
// event repo mock has no expectations, so any lookup would fail the test.
func TestEnforceTrialEventCapForNewEvent_PassThroughWhenNotCloud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := &Handler{A: &types.APIOptions{
		Cfg:         config.Configuration{},
		EventRepo:   mocks.NewMockEventRepository(ctrl),
		TrialEvents: license.NewTrialEventLimiter(nil, nil),
	}}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	require.False(t, h.enforceTrialEventCapForNewEvent(rec, req, ulid.Make().String(), "project-1", "idem-1", h.duplicateByAnyEvent))
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestEnforceTrialEventCapForNewEvent_FanoutPredicateMatchesService proves the fanout
// gate uses CreateFanoutEventService's own novelty predicate: when only duplicate-flagged
// rows exist for a key, FindFirstEventWithIdempotencyKey reports not-found, the service
// will enqueue a new non-duplicate event, and the gate must therefore consume quota (and
// 429 at the cap) instead of skipping it. A non-duplicate row for the key is a real
// duplicate and passes freely.
func TestEnforceTrialEventCapForNewEvent_FanoutPredicateMatchesService(t *testing.T) {
	client := trialTestRedis(t)
	defer client.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := ulid.Make().String()
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key:          "lk",
		Entitlements: map[string]interface{}{"daily_event_limit": 1},
	})
	require.NoError(t, err)

	org := &datastore.Organisation{UID: orgID, LicenseData: enc}
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgRepo.EXPECT().FetchOrganisationByID(gomock.Any(), orgID).Return(org, nil).AnyTimes()
	mockEventRepo := mocks.NewMockEventRepository(ctrl)

	projectID := "project-trial-fanout"
	h := &Handler{A: &types.APIOptions{
		Cfg:         config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}},
		Logger:      log.New("convoy", log.LevelInfo),
		OrgRepo:     mockOrgRepo,
		EventRepo:   mockEventRepo,
		TrialEvents: license.NewTrialEventLimiter(client, nil),
	}}

	defer client.Del(context.Background(), "trial_daily_events:"+orgID+":"+time.Now().UTC().Format("20060102"))

	do := func(key string) (bool, int) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		blocked := h.enforceTrialEventCapForNewEvent(rec, req, orgID, projectID, key, h.duplicateByFirstNonDuplicateEvent)
		return blocked, rec.Code
	}

	// Only duplicate-flagged rows exist for the key: the fanout service treats it as
	// NEW, so the gate must too — quota is consumed (limit 1 now exhausted).
	mockEventRepo.EXPECT().FindFirstEventWithIdempotencyKey(gomock.Any(), projectID, "idem-f").Return(nil, datastore.ErrEventNotFound)
	blocked, _ := do("idem-f")
	require.False(t, blocked, "duplicate-flagged-only key is new for fanout and passes under cap")

	// Same duplicate-flagged-only key at the cap: still new for fanout, so the cap IS
	// enforced (this is the uncounted-event hole the predicate alignment closes).
	mockEventRepo.EXPECT().FindFirstEventWithIdempotencyKey(gomock.Any(), projectID, "idem-f").Return(nil, datastore.ErrEventNotFound)
	blocked, code := do("idem-f")
	require.True(t, blocked, "duplicate-flagged-only key at the cap must be blocked")
	require.Equal(t, http.StatusTooManyRequests, code)

	// A non-duplicate row exists: the fanout service treats it as a duplicate, so the
	// gate skips the cap even though the counter is exhausted.
	mockEventRepo.EXPECT().FindFirstEventWithIdempotencyKey(gomock.Any(), projectID, "idem-f").Return(&datastore.Event{UID: "existing"}, nil)
	blocked, code = do("idem-f")
	require.False(t, blocked, "real duplicate at the cap must pass")
	require.NotEqual(t, http.StatusTooManyRequests, code)
}

func TestEnforceTrialEventCap_PassThroughWhenNotCloud(t *testing.T) {
	// No billing API key => not cloud => no-op even with a limiter present, so
	// self-hosted/community is unaffected and the org repo is never touched.
	a := &types.APIOptions{
		Cfg:         config.Configuration{},
		TrialEvents: license.NewTrialEventLimiter(nil, nil),
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	require.False(t, EnforceTrialEventCap(rec, req, a, ulid.Make().String()))
	require.Equal(t, http.StatusOK, rec.Code)
}
